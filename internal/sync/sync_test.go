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
)

// fakeMCP stands in for an MCP server's /sync routes. It records what it received
// and replies with canned export/import bodies.
type fakeMCP struct {
	srv            *httptest.Server
	exportBody     string // JSON returned from /sync/export
	gotExportReq   map[string]any
	gotImportReq   map[string]any
	importCalls    int
	importSizes    []int    // events received per /sync/import call
	importSuffixes []string // stream suffixes received across all /sync/import calls
	gotAuth        string
	gotWorkspace   string
}

func newFakeMCP(t *testing.T, exportBody string) *fakeMCP {
	t.Helper()
	f := &fakeMCP{exportBody: exportBody}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotAuth = r.Header.Get("Authorization")
		f.gotWorkspace = r.Header.Get("X-Workspace-Id")
		raw, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/sync/export":
			_ = json.Unmarshal(raw, &f.gotExportReq)
			_, _ = io.WriteString(w, f.exportBody)
		case "/sync/import":
			_ = json.Unmarshal(raw, &f.gotImportReq)
			// echo appended counts back per incoming stream
			var body struct {
				Streams []StreamBatch `json:"streams"`
			}
			_ = json.Unmarshal(raw, &body)
			f.importCalls++
			n := 0
			imported := make([]map[string]any, 0, len(body.Streams))
			for _, s := range body.Streams {
				n += len(s.Events)
				f.importSuffixes = append(f.importSuffixes, s.StreamSuffix)
				imported = append(imported, map[string]any{"streamSuffix": s.StreamSuffix, "appended": len(s.Events), "skipped": 0})
			}
			f.importSizes = append(f.importSizes, n)
			out, _ := json.Marshal(map[string]any{"project_id": "to-ws", "imported": imported})
			_, _ = w.Write(out)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func tmpCursors(t *testing.T) *CursorStore {
	t.Helper()
	cs, err := LoadCursors(filepath.Join(t.TempDir(), "sync-cursors.json"))
	if err != nil {
		t.Fatalf("LoadCursors: %v", err)
	}
	return cs
}

const oneDomainEvent = `{"project_id":"from-ws","streams":[{"streamSuffix":"domain","revision":5,"events":[{"correlationId":"d1"}]}]}`

func TestRunExportsFromSourceImportsToTargetWithAuthAndWorkspace(t *testing.T) {
	from := newFakeMCP(t, oneDomainEvent)
	to := newFakeMCP(t, `{}`)

	out := &bytes.Buffer{}
	err := Run(New(),
		Endpoint{MCPURL: from.srv.URL, AuthValue: "Bearer FROMTOK", Workspace: "from-ws"},
		Endpoint{MCPURL: to.srv.URL, AuthValue: "Bearer TOTOK", Workspace: "to-ws"},
		tmpCursors(t), false, out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// export hit the source with source creds; import hit the target with target creds
	if from.gotExportReq == nil {
		t.Fatal("source did not receive an export request")
	}
	if from.gotAuth != "Bearer FROMTOK" || from.gotWorkspace != "from-ws" {
		t.Fatalf("source auth/workspace = %q/%q", from.gotAuth, from.gotWorkspace)
	}
	if to.gotImportReq == nil {
		t.Fatal("target did not receive an import request")
	}
	if to.gotAuth != "Bearer TOTOK" || to.gotWorkspace != "to-ws" {
		t.Fatalf("target auth/workspace = %q/%q", to.gotAuth, to.gotWorkspace)
	}
}

func TestRunPassesStreamSuffixBatchesThroughUnchanged(t *testing.T) {
	from := newFakeMCP(t, `{"project_id":"from-ws","streams":[
		{"streamSuffix":"domain","revision":2,"events":[{"correlationId":"d1"},{"correlationId":"d2"}]},
		{"streamSuffix":"sess-1::keyframe","revision":1,"events":[{"correlationId":"k1"}]}]}`)
	to := newFakeMCP(t, `{}`)

	if err := Run(New(),
		Endpoint{MCPURL: from.srv.URL, Workspace: "from-ws"},
		Endpoint{MCPURL: to.srv.URL, Workspace: "to-ws"},
		tmpCursors(t), false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	suffixes := map[string]bool{}
	for _, s := range to.importSuffixes {
		suffixes[s] = true
	}
	if !suffixes["domain"] || !suffixes["sess-1::keyframe"] {
		t.Fatalf("suffixes not preserved across import calls: %v (suffixes seen: %v)", suffixes, to.importSuffixes)
	}
}

func TestRunPersistsCursorsKeyedBySourceThenReusesThem(t *testing.T) {
	from := newFakeMCP(t, oneDomainEvent) // revision 5
	to := newFakeMCP(t, `{}`)
	cs := tmpCursors(t)
	src := Endpoint{MCPURL: from.srv.URL, Workspace: "from-ws"}
	dst := Endpoint{MCPURL: to.srv.URL, Workspace: "to-ws"}

	if err := Run(New(), src, dst, cs, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// cursor advanced to the exported revision for this source
	if got := cs.For(cursorKey(from.srv.URL, "from-ws"))["domain"]; got != 5 {
		t.Fatalf("cursor for domain = %d, want 5", got)
	}
	// next run sends that cursor back to the source export
	if err := Run(New(), src, dst, cs, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run #2: %v", err)
	}
	cursors, _ := from.gotExportReq["cursors"].(map[string]any)
	if cursors["domain"].(float64) != 5 {
		t.Fatalf("second export did not carry cursor domain=5, got %v", cursors)
	}
}

func TestRunDryRunWritesNothingToTarget(t *testing.T) {
	from := newFakeMCP(t, oneDomainEvent)
	to := newFakeMCP(t, `{}`)
	cs := tmpCursors(t)

	out := &bytes.Buffer{}
	if err := Run(New(),
		Endpoint{MCPURL: from.srv.URL, Workspace: "from-ws"},
		Endpoint{MCPURL: to.srv.URL, Workspace: "to-ws"},
		cs, true, out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if to.gotImportReq != nil {
		t.Fatal("dry run must not import to the target")
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Fatalf("expected dry-run notice, got: %s", out.String())
	}
	// dry run must not advance cursors
	if got := cs.For(cursorKey(from.srv.URL, "from-ws"))["domain"]; got != 0 {
		t.Fatalf("dry run advanced cursor to %d", got)
	}
}

func TestRunNoNewEventsIsANoOp(t *testing.T) {
	from := newFakeMCP(t, `{"project_id":"from-ws","streams":[]}`)
	to := newFakeMCP(t, `{}`)
	out := &bytes.Buffer{}
	if err := Run(New(),
		Endpoint{MCPURL: from.srv.URL, Workspace: "from-ws"},
		Endpoint{MCPURL: to.srv.URL, Workspace: "to-ws"},
		tmpCursors(t), false, out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if to.gotImportReq != nil {
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
	body, _ := json.Marshal(map[string]any{
		"project_id": "from-ws",
		"streams":    []map[string]any{{"streamSuffix": "domain", "revision": 150, "events": events}},
	})
	from := newFakeMCP(t, string(body))
	to := newFakeMCP(t, `{}`)

	if err := Run(New(),
		Endpoint{MCPURL: from.srv.URL, Workspace: "from-ws"},
		Endpoint{MCPURL: to.srv.URL, Workspace: "to-ws"},
		tmpCursors(t), false, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if to.importCalls != 2 {
		t.Fatalf("expected 150 events to import in 2 batches, got %d calls (sizes %v)", to.importCalls, to.importSizes)
	}
	for _, n := range to.importSizes {
		if n > importBatchSize {
			t.Fatalf("a batch exceeded importBatchSize=%d: sizes %v", importBatchSize, to.importSizes)
		}
	}
	total := 0
	for _, n := range to.importSizes {
		total += n
	}
	if total != 150 {
		t.Fatalf("expected all 150 events imported, got %d", total)
	}
}

func TestSyncBaseURLStripsMcpSuffix(t *testing.T) {
	for in, want := range map[string]string{
		"https://mcp.staging.flmnt.dev/mcp": "https://mcp.staging.flmnt.dev",
		"https://mcp.staging.flmnt.dev/":    "https://mcp.staging.flmnt.dev",
		"http://localhost:8000/mcp/":        "http://localhost:8000",
		"http://localhost:8000":             "http://localhost:8000",
	} {
		if got := syncBaseURL(in); got != want {
			t.Fatalf("syncBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}
