// Package sync moves Quorum event data between environments (local <-> staging)
// through the authenticated router GraphQL (memoryExport/memoryImport). Export reads
// the source workspace's streams as project-agnostic { streamSuffix, events } batches;
// import re-homes them under the target workspace, deduping by correlationId server-side.
// Both sides authenticate through the router, which enforces the caller's rights to each
// workspace. The flow is idempotent and bidirectional-safe, so push/pull can be re-run freely.
package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// importBatchSize bounds how many events ride in one memoryImport call. The server replays Neo4j
// inline per event, so smaller batches stay well under the request timeout while keeping round-trips
// reasonable.
const importBatchSize = 100

// Endpoint is one side of a sync: an authenticated router GraphQL client, the workspace (projectId) to
// scope to, and a stable Ref (the server URL) used only to key per-source cursors.
type Endpoint struct {
	GQL       *apiclient.Client
	Workspace string
	Ref       string
}

// StreamBatch is a project-agnostic slice of one stream's raw events. Events are kept as RawMessage so
// the CLI never reshapes envelopes — it only carries them.
type StreamBatch struct {
	StreamSuffix string
	Revision     int
	Events       []json.RawMessage
}

// ImportResult is the per-stream outcome reported by the target. The QRM-ADR-016 counts are present
// only when their option was asked (reattributed on a push with attribution, reconciled / overwritten /
// kept on a pull with a collision mode, replaced on a push with replacements).
type ImportResult struct {
	StreamSuffix string   `json:"streamSuffix"`
	Appended     int      `json:"appended"`
	Skipped      int      `json:"skipped"`
	Reattributed *int     `json:"reattributed,omitempty"`
	Reconciled   *int     `json:"reconciled,omitempty"`
	Overwritten  *int     `json:"overwritten,omitempty"`
	Kept         []string `json:"kept,omitempty"`
	Replaced     *int     `json:"replaced,omitempty"`
}

// Collision is a record that exists in both stores with different content or author (QRM-ADR-016 §2):
// both envelopes as the server serialized them (JSON strings), for the person to compare.
type Collision struct {
	StreamSuffix  string `json:"streamSuffix"`
	CorrelationID string `json:"correlationId"`
	Existing      string `json:"existing"`
	Incoming      string `json:"incoming"`
}

// Resolution is the person's decision for one collision: EXISTING keeps the local copy, INCOMING
// takes the cloud copy.
type Resolution struct {
	CorrelationID string `json:"correlationId"`
	Take          string `json:"take"`
}

// Resolver decides every collision a pull reported, or refuses to.
type Resolver func(collisions []Collision) ([]Resolution, error)

// Options is what QRM-ADR-016 adds to a sync. A push carries LocalSubs (the identity this machine
// holds, re-attributed to the cloud account at the import) and replaces the records Marks hold for
// the source; a pull with a Resolver runs in collision mode — REPORT first, the Resolver decides,
// then RESOLVE — and records what was kept into Marks. Zero Options is the replication of before.
type Options struct {
	LocalSubs []string
	Marks     *MarkStore
	Resolve   Resolver
}

const exportQuery = `query($projectId: ID!, $cursors: [MemoryCursorInput!]){ memoryExport(projectId: $projectId, cursors: $cursors){ streams { streamSuffix revision events } } }`
const importMutation = `mutation($projectId: ID!, $streams: [MemoryStreamImportInput!]!){ memoryImport(projectId: $projectId, streams: $streams){ imported { streamSuffix appended skipped } } }`
const convergeMutation = `mutation($projectId: ID!, $streams: [MemoryStreamImportInput!]!, $attribution: MemoryImportAttribution, $onCollision: MemoryCollisionMode, $resolutions: [MemoryCollisionResolution!], $replace: [ID!]){ memoryImport(projectId: $projectId, streams: $streams, attribution: $attribution, onCollision: $onCollision, resolutions: $resolutions, replace: $replace){ imported { streamSuffix appended skipped reattributed reconciled overwritten kept replaced } collisions { streamSuffix correlationId existing incoming } } }`

// cursorInputs turns the suffix->afterRevision cursor map into the [{streamSuffix, after}] GraphQL
// input, key-sorted for deterministic requests.
func cursorInputs(cursors map[string]int) []map[string]any {
	keys := make([]string, 0, len(cursors))
	for k := range cursors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	in := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		in = append(in, map[string]any{"streamSuffix": k, "after": cursors[k]})
	}
	return in
}

// exportStreams runs memoryExport and decodes each stream's events (a JSON-serialized envelope array)
// into RawMessage slices for batching.
func exportStreams(ep Endpoint, cursors map[string]int) ([]StreamBatch, error) {
	var data struct {
		MemoryExport struct {
			Streams []struct {
				StreamSuffix string `json:"streamSuffix"`
				Revision     int    `json:"revision"`
				Events       string `json:"events"`
			} `json:"streams"`
		} `json:"memoryExport"`
	}
	if err := ep.GQL.Query(exportQuery, map[string]any{"projectId": ep.Workspace, "cursors": cursorInputs(cursors)}, &data); err != nil {
		return nil, err
	}
	out := make([]StreamBatch, 0, len(data.MemoryExport.Streams))
	for _, s := range data.MemoryExport.Streams {
		var events []json.RawMessage
		if err := json.Unmarshal([]byte(s.Events), &events); err != nil {
			return nil, fmt.Errorf("decoding events for %s: %w", s.StreamSuffix, err)
		}
		out = append(out, StreamBatch{StreamSuffix: s.StreamSuffix, Revision: s.Revision, Events: events})
	}
	return out, nil
}

// importArgs is what one memoryImport call asks beyond the batch: nil fields are not sent.
type importArgs struct {
	localSubs   []string
	onCollision string
	resolutions []Resolution
	replace     []string
}

func (a importArgs) converges() bool {
	return len(a.localSubs) > 0 || a.onCollision != "" || len(a.replace) > 0
}

type importReply struct {
	Imported   []ImportResult `json:"imported"`
	Collisions []Collision    `json:"collisions"`
}

// importBatch runs memoryImport for one stream's batch, re-serializing the events to the JSON string
// the mutation expects. The plain mutation is sent unless QRM-ADR-016's arguments are in play, so a
// server that predates them still serves a plain replication.
func importBatch(ep Endpoint, suffix string, events []json.RawMessage, args importArgs) (importReply, error) {
	eventsStr, err := json.Marshal(events)
	if err != nil {
		return importReply{}, err
	}
	var data struct {
		MemoryImport importReply `json:"memoryImport"`
	}
	variables := map[string]any{"projectId": ep.Workspace, "streams": []map[string]any{{"streamSuffix": suffix, "events": string(eventsStr)}}}
	mutation := importMutation
	if args.converges() {
		mutation = convergeMutation
		if len(args.localSubs) > 0 {
			variables["attribution"] = map[string]any{"localSubs": args.localSubs}
		}
		if args.onCollision != "" {
			variables["onCollision"] = args.onCollision
			variables["resolutions"] = resolutionsOrEmpty(args.resolutions)
		}
		if len(args.replace) > 0 {
			variables["replace"] = args.replace
		}
	}
	if err := ep.GQL.Query(mutation, variables, &data); err != nil {
		return importReply{}, err
	}
	return data.MemoryImport, nil
}

func resolutionsOrEmpty(r []Resolution) []Resolution {
	if r == nil {
		return []Resolution{}
	}
	return r
}

// Plan is the preview of what a sync would move, per stream.
type Plan struct {
	Streams []StreamBatch
}

// Total returns the number of events the plan would move.
func (p Plan) Total() int {
	n := 0
	for _, s := range p.Streams {
		n += len(s.Events)
	}
	return n
}

// Run executes one sync: export from `from` (honoring saved cursors), and unless dryRun, import into
// `to`. On success it advances and persists the per-source cursors so the next run moves only new
// events. Progress is written to `out`.
func Run(from, to Endpoint, cursors *CursorStore, dryRun bool, out io.Writer) error {
	return RunWith(from, to, cursors, dryRun, out, Options{})
}

// RunWith is Run with QRM-ADR-016's Options (see Options).
func RunWith(from, to Endpoint, cursors *CursorStore, dryRun bool, out io.Writer, opts Options) error {
	key := cursorKey(from.Ref, from.Workspace)
	streams, err := exportStreams(from, cursors.For(key))
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	plan := Plan{Streams: streams}
	if plan.Total() == 0 {
		fmt.Fprintln(out, "Nothing to sync — already up to date.")
		return nil
	}

	fmt.Fprintf(out, "Will move %d record(s) across %d collection(s):\n", plan.Total(), len(plan.Streams))
	for _, s := range plan.Streams {
		fmt.Fprintf(out, "  %-32s %d record(s)\n", s.StreamSuffix, len(s.Events))
	}
	if dryRun {
		fmt.Fprintln(out, "(dry run — nothing written)")
		return nil
	}

	// A pull in collision mode asks the target to REPORT first — nothing is written — so every
	// twin that differs is decided before the import that writes (QRM-ADR-016 §2).
	resolutionsOf := map[string][]Resolution{}
	if opts.Resolve != nil {
		collisions, err := reportCollisions(to, streams)
		if err != nil {
			return err
		}
		if len(collisions) > 0 {
			fmt.Fprintf(out, "%d record(s) exist in both places with different content.\n", len(collisions))
			resolutions, err := opts.Resolve(collisions)
			if err != nil {
				return err
			}
			streamOf := map[string]string{}
			for _, c := range collisions {
				streamOf[c.CorrelationID] = c.StreamSuffix
			}
			for _, r := range resolutions {
				suffix := streamOf[r.CorrelationID]
				resolutionsOf[suffix] = append(resolutionsOf[suffix], r)
			}
		}
	}

	// The local side's marks (records the person kept as truth on a pull) are what a push replaces.
	markKey := cursorKey(from.Ref, from.Workspace)

	// Import one stream at a time, in bounded batches. Import replays Neo4j inline per event, so a
	// single huge request would blow the timeout; chunking keeps each request fast. Dedup by
	// correlationId on the server makes the chunks safe to re-send.
	for _, s := range streams {
		var replace []string
		if opts.Marks != nil && opts.Resolve == nil {
			replace = opts.Marks.For(markKey, s.StreamSuffix)
		}
		for start := 0; start < len(s.Events); start += importBatchSize {
			end := start + importBatchSize
			if end > len(s.Events) {
				end = len(s.Events)
			}
			args := importArgs{localSubs: opts.LocalSubs, replace: replace}
			if opts.Resolve != nil {
				args.onCollision = "RESOLVE"
				args.resolutions = resolutionsOf[s.StreamSuffix]
			}
			reply, err := importBatch(to, s.StreamSuffix, s.Events[start:end], args)
			if err != nil {
				return fmt.Errorf("import %s: %w", s.StreamSuffix, err)
			}
			for _, r := range reply.Imported {
				fmt.Fprintln(out, importLine(r))
				if opts.Marks != nil && opts.Resolve != nil {
					opts.Marks.Add(cursorKey(to.Ref, to.Workspace), r.StreamSuffix, r.Kept)
				}
			}
		}
		if opts.Marks != nil && len(replace) > 0 {
			opts.Marks.Clear(markKey, s.StreamSuffix, replace)
		}
		// Advance this source stream's cursor only after all its batches landed, so re-runs skip what
		// we've moved. Import dedups by correlationId as backstop.
		if s.Revision > 0 {
			cursors.Set(key, s.StreamSuffix, s.Revision)
		}
	}

	if err := cursors.Save(); err != nil {
		return fmt.Errorf("saving cursors: %w", err)
	}
	if opts.Marks != nil {
		if err := opts.Marks.Save(); err != nil {
			return fmt.Errorf("saving marks: %w", err)
		}
	}
	return nil
}

// reportCollisions asks the target what differs, stream by stream, writing nothing.
func reportCollisions(to Endpoint, streams []StreamBatch) ([]Collision, error) {
	var all []Collision
	for _, s := range streams {
		for start := 0; start < len(s.Events); start += importBatchSize {
			end := start + importBatchSize
			if end > len(s.Events) {
				end = len(s.Events)
			}
			reply, err := importBatch(to, s.StreamSuffix, s.Events[start:end], importArgs{onCollision: "REPORT"})
			if err != nil {
				return nil, fmt.Errorf("compare %s: %w", s.StreamSuffix, err)
			}
			all = append(all, reply.Collisions...)
		}
	}
	return all, nil
}

// importLine is one stream's outcome, with QRM-ADR-016's counts when the import carried them.
func importLine(r ImportResult) string {
	line := fmt.Sprintf("  %-32s appended %d, skipped %d", r.StreamSuffix, r.Appended, r.Skipped)
	if r.Reattributed != nil {
		line += fmt.Sprintf(", re-attributed %d", *r.Reattributed)
	}
	if r.Reconciled != nil {
		line += fmt.Sprintf(", reconciled %d", *r.Reconciled)
	}
	if r.Overwritten != nil {
		line += fmt.Sprintf(", overwritten %d", *r.Overwritten)
	}
	if len(r.Kept) > 0 {
		line += fmt.Sprintf(", kept %d", len(r.Kept))
	}
	if r.Replaced != nil {
		line += fmt.Sprintf(", replaced %d", *r.Replaced)
	}
	return line
}
