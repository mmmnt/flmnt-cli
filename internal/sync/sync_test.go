package sync

import (
	"bytes"
	"encoding/json"
	"errors"
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

/* QRM-ADR-016: the fake router with the convergence arguments — what a REPORT answers, what RESOLVE records. */
type convergeRouter struct {
	srv         *httptest.Server
	collisions  []map[string]any
	calls       []map[string]any
	keptOnWrite []string
}

func newConvergeRouter(t *testing.T, exportBody string, collisions []map[string]any, kept []string) *convergeRouter {
	t.Helper()
	f := &convergeRouter{collisions: collisions, keptOnWrite: kept}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &req)
		if strings.Contains(req.Query, "memoryExport") {
			_, _ = io.WriteString(w, exportBody)
			return
		}
		f.calls = append(f.calls, req.Variables)
		streams, _ := req.Variables["streams"].([]any)
		sm, _ := streams[0].(map[string]any)
		suffix, _ := sm["streamSuffix"].(string)
		var events []json.RawMessage
		_ = json.Unmarshal([]byte(sm["events"].(string)), &events)
		mode, _ := req.Variables["onCollision"].(string)
		result := map[string]any{"streamSuffix": suffix, "appended": len(events), "skipped": 0}
		var collisions []map[string]any
		switch mode {
		case "REPORT":
			result["appended"] = 0
			collisions = f.collisions
		case "RESOLVE":
			result["overwritten"] = len(f.collisions) - len(f.keptOnWrite)
			result["kept"] = f.keptOnWrite
		}
		if rep, ok := req.Variables["replace"].([]any); ok {
			result["replaced"] = len(rep)
		}
		if collisions == nil {
			collisions = []map[string]any{}
		}
		out, _ := json.Marshal(map[string]any{"data": map[string]any{"memoryImport": map[string]any{"imported": []map[string]any{result}, "collisions": collisions}}})
		_, _ = w.Write(out)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *convergeRouter) endpoint(ws string) Endpoint {
	return Endpoint{GQL: apiclient.New(f.srv.URL, "t"), Workspace: ws, Ref: f.srv.URL}
}

func tmpMarks(t *testing.T) *MarkStore {
	t.Helper()
	m, err := LoadMarks(filepath.Join(t.TempDir(), "sync-marks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

/*
A push carries the identity this machine holds as `identity.localSubs` (QRM-ADR-016 §1, SPEC-LINK-001:
linked to the cloud account, never rewritten onto a record) and, as

	`replace`, the records the person kept as local truth on an earlier pull (ruling 7); a replaced
	mark is cleared. Without either, the plain mutation of before is sent.
*/
func TestRunWithPushCarriesIdentityAndReplacesKeptMarks(t *testing.T) {
	src := newFakeRouter(t, exportData(oneDomainStream))
	dst := newConvergeRouter(t, "", nil, nil)
	marks := tmpMarks(t)
	marks.Add(cursorKey(src.srv.URL, "quorum"), "domain", []string{"d1"})
	var out bytes.Buffer

	err := RunWith(src.endpoint("quorum", "local-token"), dst.endpoint("ws-cloud"), tmpCursors(t), false, &out, Options{LocalSubs: []string{"local_34f6"}, Marks: marks})
	if err != nil {
		t.Fatal(err)
	}
	if len(dst.calls) != 1 {
		t.Fatalf("one import call, got %d", len(dst.calls))
	}
	vars := dst.calls[0]
	if id, _ := vars["identity"].(map[string]any); id == nil || id["localSubs"].([]any)[0] != "local_34f6" {
		t.Fatalf("identity not sent: %v", vars)
	}
	if rep, _ := vars["replace"].([]any); len(rep) != 1 || rep[0] != "d1" {
		t.Fatalf("replace not sent: %v", vars)
	}
	if _, has := vars["onCollision"]; has {
		t.Fatalf("a push asks no collision mode: %v", vars)
	}
	if got := marks.For(cursorKey(src.srv.URL, "quorum"), "domain"); len(got) != 0 {
		t.Fatalf("a replaced mark is cleared, got %v", got)
	}
	if strings.Contains(out.String(), "re-attributed") || !strings.Contains(out.String(), "replaced 1") {
		t.Fatalf("the line carries the replace count and no re-attribution: %q", out.String())
	}
}

/*
A pull in collision mode asks REPORT first (nothing written), hands every collision to the Resolver,

	then RESOLVEs with its decisions, filed under the stream each record belongs to; what the server
	kept is marked for the next push.
*/
func TestRunWithPullReportsResolvesAndMarksWhatWasKept(t *testing.T) {
	src := newFakeRouter(t, exportData([]streamDef{{suffix: "domain", revision: 3, events: []map[string]any{{"correlationId": "d1"}, {"correlationId": "d2"}}}}))
	collisions := []map[string]any{
		{"streamSuffix": "domain", "correlationId": "d1", "existing": `{"payload":{"content":"local"}}`, "incoming": `{"payload":{"content":"cloud"}}`},
		{"streamSuffix": "domain", "correlationId": "d2", "existing": `{"payload":{"content":"a"}}`, "incoming": `{"payload":{"content":"b"}}`},
	}
	dst := newConvergeRouter(t, "", collisions, []string{"d2"})
	marks := tmpMarks(t)
	var decided []Collision
	resolve := func(cs []Collision) ([]Resolution, error) {
		decided = cs
		return []Resolution{{CorrelationID: "d1", Take: "INCOMING"}, {CorrelationID: "d2", Take: "EXISTING"}}, nil
	}
	var out bytes.Buffer

	err := RunWith(src.endpoint("ws-cloud", "hosted"), dst.endpoint("quorum"), tmpCursors(t), false, &out, Options{Marks: marks, Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}
	if len(decided) != 2 || decided[0].Existing != `{"payload":{"content":"local"}}` {
		t.Fatalf("the resolver sees both envelopes of every collision: %+v", decided)
	}
	if len(dst.calls) != 2 || dst.calls[0]["onCollision"] != "REPORT" || dst.calls[1]["onCollision"] != "RESOLVE" {
		t.Fatalf("REPORT then RESOLVE, got %d calls: %v", len(dst.calls), dst.calls)
	}
	res, _ := dst.calls[1]["resolutions"].([]any)
	if len(res) != 2 {
		t.Fatalf("both resolutions sent with the stream's RESOLVE: %v", dst.calls[1])
	}
	if got := marks.For(cursorKey(dst.srv.URL, "quorum"), "domain"); len(got) != 1 || got[0] != "d2" {
		t.Fatalf("what the server kept is marked for the next push, got %v", got)
	}
	for _, want := range []string{"2 record(s) exist in both places", "overwritten 1", "kept 1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("want %q in %q", want, out.String())
		}
	}
}

/* With nothing colliding, the pull still RESOLVEs (REPORT wrote nothing) and the resolver is never asked. */
func TestRunWithPullWithoutCollisionsNeverAsksAndStillWrites(t *testing.T) {
	src := newFakeRouter(t, exportData(oneDomainStream))
	dst := newConvergeRouter(t, "", nil, nil)
	asked := false
	var out bytes.Buffer
	err := RunWith(src.endpoint("ws-cloud", "hosted"), dst.endpoint("quorum"), tmpCursors(t), false, &out, Options{Marks: tmpMarks(t), Resolve: func([]Collision) ([]Resolution, error) { asked = true; return nil, nil }})
	if err != nil || asked {
		t.Fatalf("err %v asked %v", err, asked)
	}
	if len(dst.calls) != 2 || dst.calls[1]["onCollision"] != "RESOLVE" {
		t.Fatalf("got %v", dst.calls)
	}
	if res, ok := dst.calls[1]["resolutions"].([]any); !ok || len(res) != 0 {
		t.Fatalf("an empty resolutions list rides with RESOLVE: %v", dst.calls[1])
	}
}

/* A resolver that declines stops the pull before anything is written. */
func TestRunWithPullStopsWhenTheResolverDeclines(t *testing.T) {
	src := newFakeRouter(t, exportData(oneDomainStream))
	dst := newConvergeRouter(t, "", []map[string]any{{"streamSuffix": "domain", "correlationId": "d1", "existing": `{}`, "incoming": `{}`}}, nil)
	var out bytes.Buffer
	err := RunWith(src.endpoint("ws-cloud", "hosted"), dst.endpoint("quorum"), tmpCursors(t), false, &out, Options{Resolve: func([]Collision) ([]Resolution, error) { return nil, errors.New("no terminal; pass --take") }})
	if err == nil || !strings.Contains(err.Error(), "--take") || len(dst.calls) != 1 {
		t.Fatalf("err %v calls %d", err, len(dst.calls))
	}
}
