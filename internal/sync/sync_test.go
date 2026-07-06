package sync

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// fakeRouter stands in for the router GraphQL memory subgraph. It records what it received and replies
// with canned memoryExport/memoryImport payloads.
type fakeRouter struct {
	srv            *httptest.Server
	exportBody     string
	gotExportVars  map[string]any
	gotImportVars  map[string]any
	importCalls    int
	importSizes    []int
	importSuffixes []string
	gotAuth        string
}

type streamDef struct {
	suffix   string
	revision int
	events   []map[string]any
}

// exportData builds a memoryExport GraphQL response, serializing each stream's events into the JSON
// string the real resolver returns.
func exportData(streams []streamDef) string {
	type gs struct {
		StreamSuffix string `json:"streamSuffix"`
		Revision     int    `json:"revision"`
		Events       string `json:"events"`
	}
	out := make([]gs, 0, len(streams))
	for _, s := range streams {
		ev, _ := json.Marshal(s.events)
		out = append(out, gs{s.suffix, s.revision, string(ev)})
	}
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"memoryExport": map[string]any{"streams": out}}})
	return string(body)
}

func newFakeRouter(t *testing.T, exportBody string) *fakeRouter {
	t.Helper()
	f := &fakeRouter{exportBody: exportBody}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &req)
		switch {
		case strings.Contains(req.Query, "memoryExport"):
			f.gotExportVars = req.Variables
			_, _ = io.WriteString(w, f.exportBody)
		case strings.Contains(req.Query, "memoryImport"):
			f.gotImportVars = req.Variables
			f.importCalls++
			streams, _ := req.Variables["streams"].([]any)
			imported := make([]map[string]any, 0, len(streams))
			total := 0
			for _, s := range streams {
				sm, _ := s.(map[string]any)
				suffix, _ := sm["streamSuffix"].(string)
				eventsStr, _ := sm["events"].(string)
				var events []json.RawMessage
				_ = json.Unmarshal([]byte(eventsStr), &events)
				total += len(events)
				f.importSuffixes = append(f.importSuffixes, suffix)
				imported = append(imported, map[string]any{"streamSuffix": suffix, "appended": len(events), "skipped": 0})
			}
			f.importSizes = append(f.importSizes, total)
			out, _ := json.Marshal(map[string]any{"data": map[string]any{"memoryImport": map[string]any{"imported": imported}}})
			_, _ = w.Write(out)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRouter) endpoint(ws, token string) Endpoint {
	return Endpoint{GQL: apiclient.New(f.srv.URL, token), Workspace: ws, Ref: f.srv.URL}
}

func tmpCursors(t *testing.T) *CursorStore {
	t.Helper()
	cs, err := LoadCursors(filepath.Join(t.TempDir(), "sync-cursors.json"))
	if err != nil {
		t.Fatalf("LoadCursors: %v", err)
	}
	return cs
}

var oneDomainStream = []streamDef{{suffix: "domain", revision: 5, events: []map[string]any{{"correlationId": "d1"}}}}

func TestRunExportsFromSourceImportsToTargetAuthenticatedPerSide(t *testing.T) {
	from := newFakeRouter(t, exportData(oneDomainStream))
	to := newFakeRouter(t, exportData(nil))

	err := Run(from.endpoint("from-ws", "FROMTOK"), to.endpoint("to-ws", "TOTOK"), tmpCursors(t), false, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if from.gotExportVars == nil || from.gotExportVars["projectId"] != "from-ws" {
		t.Fatalf("source export vars: %#v", from.gotExportVars)
	}
	if from.gotAuth != "Bearer FROMTOK" {
		t.Fatalf("source auth = %q", from.gotAuth)
	}
	if to.gotImportVars == nil || to.gotImportVars["projectId"] != "to-ws" {
		t.Fatalf("target import vars: %#v", to.gotImportVars)
	}
	if to.gotAuth != "Bearer TOTOK" {
		t.Fatalf("target auth = %q", to.gotAuth)
	}
}

func TestRunPassesStreamSuffixesThroughUnchanged(t *testing.T) {
	from := newFakeRouter(t, exportData([]streamDef{
		{suffix: "domain", revision: 2, events: []map[string]any{{"correlationId": "d1"}, {"correlationId": "d2"}}},
		{suffix: "sess-1::keyframe", revision: 1, events: []map[string]any{{"correlationId": "k1"}}},
	}))
	to := newFakeRouter(t, exportData(nil))

	if err := Run(from.endpoint("from-ws", ""), to.endpoint("to-ws", ""), tmpCursors(t), false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	seen := map[string]bool{}
	for _, s := range to.importSuffixes {
		seen[s] = true
	}
	if !seen["domain"] || !seen["sess-1::keyframe"] {
		t.Fatalf("suffixes not preserved: %v", to.importSuffixes)
	}
}

func TestRunPersistsCursorsKeyedBySourceThenReusesThem(t *testing.T) {
	from := newFakeRouter(t, exportData(oneDomainStream))
	to := newFakeRouter(t, exportData(nil))
	cs := tmpCursors(t)
	src := from.endpoint("from-ws", "")
	dst := to.endpoint("to-ws", "")

	if err := Run(src, dst, cs, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := cs.For(cursorKey(from.srv.URL, "from-ws"))["domain"]; got != 5 {
		t.Fatalf("cursor for domain = %d, want 5", got)
	}
	if err := Run(src, dst, cs, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run #2: %v", err)
	}
	cursors, _ := from.gotExportVars["cursors"].([]any)
	found := false
	for _, c := range cursors {
		cm, _ := c.(map[string]any)
		if cm["streamSuffix"] == "domain" && cm["after"].(float64) == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("second export did not carry cursor domain=5, got %v", cursors)
	}
}

func TestRunDryRunWritesNothingToTarget(t *testing.T) {
	from := newFakeRouter(t, exportData(oneDomainStream))
	to := newFakeRouter(t, exportData(nil))
	cs := tmpCursors(t)

	out := &bytes.Buffer{}
	if err := Run(from.endpoint("from-ws", ""), to.endpoint("to-ws", ""), cs, true, out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if to.gotImportVars != nil {
		t.Fatal("dry run must not import to the target")
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Fatalf("expected dry-run notice, got: %s", out.String())
	}
	if got := cs.For(cursorKey(from.srv.URL, "from-ws"))["domain"]; got != 0 {
		t.Fatalf("dry run advanced cursor to %d", got)
	}
}

func TestRunNoNewEventsIsANoOp(t *testing.T) {
	from := newFakeRouter(t, exportData(nil))
	to := newFakeRouter(t, exportData(nil))
	out := &bytes.Buffer{}
	if err := Run(from.endpoint("from-ws", ""), to.endpoint("to-ws", ""), tmpCursors(t), false, out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if to.gotImportVars != nil {
		t.Fatal("no-op sync must not import")
	}
	if !strings.Contains(out.String(), "Nothing to sync") {
		t.Fatalf("expected nothing-to-sync notice, got: %s", out.String())
	}
}

func TestRunChunksLargeStreamImportsIntoBatches(t *testing.T) {
	events := make([]map[string]any, 0, 150)
	for i := 0; i < 150; i++ {
		events = append(events, map[string]any{"correlationId": i})
	}
	from := newFakeRouter(t, exportData([]streamDef{{suffix: "domain", revision: 150, events: events}}))
	to := newFakeRouter(t, exportData(nil))

	if err := Run(from.endpoint("from-ws", ""), to.endpoint("to-ws", ""), tmpCursors(t), false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if to.importCalls != 2 {
		t.Fatalf("expected 150 events in 2 batches, got %d calls (sizes %v)", to.importCalls, to.importSizes)
	}
	total := 0
	for _, n := range to.importSizes {
		if n > importBatchSize {
			t.Fatalf("a batch exceeded importBatchSize=%d: sizes %v", importBatchSize, to.importSizes)
		}
		total += n
	}
	if total != 150 {
		t.Fatalf("expected all 150 events imported, got %d", total)
	}
}
