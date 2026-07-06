package derive

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// TestWriteSessionRunsMemoryImportAuthenticated verifies the writer's contract: it runs the
// memoryImport mutation through the router with a bearer token, scopes to the workspace via projectId,
// and routes decisions→domain, mistakes→mistake with events carried as a JSON string.
func TestWriteSessionRunsMemoryImportAuthenticated(t *testing.T) {
	var gotAuth, gotQuery string
	var vars map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &req)
		gotQuery = req.Query
		vars = req.Variables
		_, _ = w.Write([]byte(`{"data":{"memoryImport":{"imported":[{"streamSuffix":"domain","appended":1,"skipped":0}]}}}`))
	}))
	defer srv.Close()

	w := &Writer{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "ws-1"}
	d := SessionDerivation{
		SessionID: "s1",
		WindowTo:  "2026-06-20T00:00:00Z",
		Candidates: []Candidate{
			{Kind: KindDecision, LocalID: "d1", Title: "Decision", Text: "Use the router read op."},
			{Kind: KindMistake, LocalID: "m1", Title: "Mistake", Text: "Pointed at the wrong workspace."},
		},
	}
	res, err := w.WriteSession(d)
	if err != nil {
		t.Fatalf("WriteSession: %v", err)
	}

	if !strings.Contains(gotQuery, "memoryImport") {
		t.Errorf("query = %q, want memoryImport", gotQuery)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want Bearer tok", gotAuth)
	}
	if vars["projectId"] != "ws-1" {
		t.Errorf("projectId = %v, want ws-1", vars["projectId"])
	}
	streams, ok := vars["streams"].([]any)
	if !ok || len(streams) != 2 {
		t.Fatalf("want 2 stream batches (domain+mistake), got %v", vars["streams"])
	}
	suffixes := map[string]bool{}
	for _, s := range streams {
		m, _ := s.(map[string]any)
		suffixes[m["streamSuffix"].(string)] = true
		if _, isString := m["events"].(string); !isString {
			t.Errorf("events must be a JSON string, got %T", m["events"])
		}
	}
	if !suffixes["domain"] || !suffixes["mistake"] {
		t.Errorf("stream suffixes = %v, want domain+mistake", suffixes)
	}
	if res.Appended != 1 {
		t.Errorf("Appended = %d, want 1", res.Appended)
	}
}

// TestWriteSessionOmitsAuthForLocalStack confirms a bare local stack (empty token) sends no
// Authorization header — the writer's "" auth path.
func TestWriteSessionOmitsAuthForLocalStack(t *testing.T) {
	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(`{"data":{"memoryImport":{"imported":[]}}}`))
	}))
	defer srv.Close()

	w := &Writer{GQL: apiclient.New(srv.URL, "")} // no token
	if _, err := w.WriteSession(SessionDerivation{
		Candidates: []Candidate{{Kind: KindDecision, LocalID: "d1", Text: "x"}},
	}); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	if hadAuth {
		t.Error("Authorization should be absent when the token is empty")
	}
}

// TestWriteSessionDryRunLogsWithoutWriting confirms dry-run previews without calling the router.
func TestWriteSessionDryRunLogsWithoutWriting(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	var logged string
	w := &Writer{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "ws-1", DryRun: true, Log: func(s string) { logged = s }}
	res, err := w.WriteSession(SessionDerivation{Candidates: []Candidate{{Kind: KindDecision, LocalID: "d1", Text: "x"}}})
	if err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	if called {
		t.Error("dry run must not call the router")
	}
	if res.Appended != 1 || !strings.Contains(logged, "domain") {
		t.Fatalf("dry-run result/preview wrong: appended=%d log=%s", res.Appended, logged)
	}
}
