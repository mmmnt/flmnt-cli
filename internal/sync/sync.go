// Package sync moves Quorum event data between environments (local <-> staging)
// through their public MCP /sync routes. Export reads the source workspace's
// streams as project-agnostic { streamSuffix, events } batches; import re-homes
// them under the target workspace, deduping by correlationId server-side. The
// flow is idempotent and bidirectional-safe, so push/pull can be re-run freely.
package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// importBatchSize bounds how many events ride in one /sync/import request. The
// server replays Neo4j inline per event, so smaller batches stay well under the
// upstream request timeout while keeping round-trips reasonable.
const importBatchSize = 100

// Endpoint is one side of a sync: an MCP base URL, a ready Authorization header
// value (e.g. "Bearer <token>"), and the workspace id to scope to.
type Endpoint struct {
	MCPURL    string
	AuthValue string
	Workspace string
}

// syncBaseURL normalizes an MCP URL to the base the /sync routes live on. The
// MCP protocol endpoint is "<base>/mcp"; the sync routes are at "<base>/sync/*".
func syncBaseURL(raw string) string {
	u := strings.TrimRight(raw, "/")
	u = strings.TrimSuffix(u, "/mcp")
	return strings.TrimRight(u, "/")
}

// StreamBatch is a project-agnostic slice of one stream's raw events. Events are
// kept as RawMessage so the CLI never reshapes envelopes — it only carries them.
type StreamBatch struct {
	StreamSuffix string            `json:"streamSuffix"`
	Revision     int               `json:"revision,omitempty"`
	Events       []json.RawMessage `json:"events"`
}

type exportResponse struct {
	ProjectID string        `json:"project_id"`
	Streams   []StreamBatch `json:"streams"`
}

// ImportResult is the per-stream outcome reported by the target.
type ImportResult struct {
	StreamSuffix string `json:"streamSuffix"`
	Appended     int    `json:"appended"`
	Skipped      int    `json:"skipped"`
}

type importResponse struct {
	ProjectID string         `json:"project_id"`
	Imported  []ImportResult `json:"imported"`
}

// Client performs the HTTP calls against the MCP /sync routes.
type Client struct {
	HTTP *http.Client
}

// New returns a Client with a sane default timeout.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) post(ep Endpoint, path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := syncBaseURL(ep.MCPURL) + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if ep.AuthValue != "" {
		req.Header.Set("Authorization", ep.AuthValue)
	}
	if ep.Workspace != "" {
		req.Header.Set("X-Workspace-Id", ep.Workspace)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s -> HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(payload, out)
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

// Run executes one sync: export from `from` (honoring saved cursors), and unless
// dryRun, import into `to`. On success it advances and persists the per-source
// cursors so the next run moves only new events. Progress is written to `out`.
func Run(c *Client, from, to Endpoint, cursors *CursorStore, dryRun bool, out io.Writer) error {
	key := cursorKey(from.MCPURL, from.Workspace)
	var exp exportResponse
	if err := c.post(from, "/sync/export", map[string]any{"cursors": cursors.For(key)}, &exp); err != nil {
		return fmt.Errorf("export: %w", err)
	}

	plan := Plan{Streams: exp.Streams}
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

	// Import one stream at a time, in bounded batches. The server's import replays
	// Neo4j inline per event, so a single huge request would blow the upstream
	// timeout; chunking keeps each request fast. Dedup by correlationId on the
	// server makes the chunks safe to re-send.
	for _, s := range exp.Streams {
		for start := 0; start < len(s.Events); start += importBatchSize {
			end := start + importBatchSize
			if end > len(s.Events) {
				end = len(s.Events)
			}
			batch := StreamBatch{StreamSuffix: s.StreamSuffix, Events: s.Events[start:end]}
			var res importResponse
			if err := c.post(to, "/sync/import", map[string]any{"streams": []StreamBatch{batch}}, &res); err != nil {
				return fmt.Errorf("import %s: %w", s.StreamSuffix, err)
			}
			for _, r := range res.Imported {
				fmt.Fprintf(out, "  %-32s appended %d, skipped %d\n", r.StreamSuffix, r.Appended, r.Skipped)
			}
		}
		// Advance this source stream's cursor only after all its batches landed, so
		// re-runs skip what we've moved. Import dedups by correlationId as backstop.
		if s.Revision > 0 {
			cursors.Set(key, s.StreamSuffix, s.Revision)
		}
	}

	if err := cursors.Save(); err != nil {
		return fmt.Errorf("saving cursors: %w", err)
	}
	return nil
}
