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

// ImportResult is the per-stream outcome reported by the target.
type ImportResult struct {
	StreamSuffix string `json:"streamSuffix"`
	Appended     int    `json:"appended"`
	Skipped      int    `json:"skipped"`
}

const exportQuery = `query($projectId: ID!, $cursors: [MemoryCursorInput!]){ memoryExport(projectId: $projectId, cursors: $cursors){ streams { streamSuffix revision events } } }`
const importMutation = `mutation($projectId: ID!, $streams: [MemoryStreamImportInput!]!){ memoryImport(projectId: $projectId, streams: $streams){ imported { streamSuffix appended skipped } } }`

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

// importBatch runs memoryImport for one stream's batch, re-serializing the events to the JSON string
// the mutation expects.
func importBatch(ep Endpoint, suffix string, events []json.RawMessage) ([]ImportResult, error) {
	eventsStr, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}
	var data struct {
		MemoryImport struct {
			Imported []ImportResult `json:"imported"`
		} `json:"memoryImport"`
	}
	streams := []map[string]any{{"streamSuffix": suffix, "events": string(eventsStr)}}
	if err := ep.GQL.Query(importMutation, map[string]any{"projectId": ep.Workspace, "streams": streams}, &data); err != nil {
		return nil, err
	}
	return data.MemoryImport.Imported, nil
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

	// Import one stream at a time, in bounded batches. Import replays Neo4j inline per event, so a
	// single huge request would blow the timeout; chunking keeps each request fast. Dedup by
	// correlationId on the server makes the chunks safe to re-send.
	for _, s := range streams {
		for start := 0; start < len(s.Events); start += importBatchSize {
			end := start + importBatchSize
			if end > len(s.Events) {
				end = len(s.Events)
			}
			imported, err := importBatch(to, s.StreamSuffix, s.Events[start:end])
			if err != nil {
				return fmt.Errorf("import %s: %w", s.StreamSuffix, err)
			}
			for _, r := range imported {
				fmt.Fprintf(out, "  %-32s appended %d, skipped %d\n", r.StreamSuffix, r.Appended, r.Skipped)
			}
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
	return nil
}
