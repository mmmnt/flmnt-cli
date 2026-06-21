package derive

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWriteSessionPostsImportWithWorkspaceHeader verifies the writer's remote contract: it targets
// /sync/import (stripping a /mcp suffix), sends Authorization + X-Workspace-Id (the proxy requires
// workspace selection), and routes decisions→domain, mistakes→mistake.
func TestWriteSessionPostsImportWithWorkspaceHeader(t *testing.T) {
	var gotPath, gotAuth, gotWorkspace string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotWorkspace = r.Header.Get("X-Workspace-Id")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"imported": []map[string]any{{"streamSuffix": "domain", "appended": 1, "skipped": 0}},
		})
	}))
	defer srv.Close()

	w := &Writer{
		Endpoint:   srv.URL + "/mcp", // /mcp must be stripped to reach /sync/import
		ProjectID:  "ws-1",
		AuthHeader: "Bearer tok",
		HTTP:       srv.Client(),
	}
	d := SessionDerivation{
		SessionID: "s1",
		WindowTo:  "2026-06-20T00:00:00Z",
		Candidates: []Candidate{
			{Kind: KindDecision, LocalID: "d1", Title: "Decision", Text: "Use the proxy read route."},
			{Kind: KindMistake, LocalID: "m1", Title: "Mistake", Text: "Pointed at the wrong workspace."},
		},
	}
	res, err := w.WriteSession(d)
	if err != nil {
		t.Fatalf("WriteSession: %v", err)
	}

	if gotPath != "/sync/import" {
		t.Errorf("path = %q, want /sync/import (was /mcp stripped?)", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok")
	}
	if gotWorkspace != "ws-1" {
		t.Errorf("X-Workspace-Id = %q, want %q", gotWorkspace, "ws-1")
	}
	if body["project_id"] != "ws-1" {
		t.Errorf("body project_id = %v, want ws-1", body["project_id"])
	}
	streams, ok := body["streams"].([]any)
	if !ok || len(streams) != 2 {
		t.Fatalf("want 2 stream batches (domain+mistake), got %v", body["streams"])
	}
	suffixes := map[string]bool{}
	for _, s := range streams {
		if m, ok := s.(map[string]any); ok {
			suffixes[m["streamSuffix"].(string)] = true
		}
	}
	if !suffixes["domain"] || !suffixes["mistake"] {
		t.Errorf("stream suffixes = %v, want domain+mistake", suffixes)
	}
	if res.Appended != 1 {
		t.Errorf("Appended = %d, want 1", res.Appended)
	}
}

// TestWriteSessionOmitsWorkspaceHeaderForLocalStack confirms a bare local stack (no ProjectID/auth)
// sends neither header — the writer's "" auth path.
func TestWriteSessionOmitsWorkspaceHeaderForLocalStack(t *testing.T) {
	var hadWorkspace, hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadWorkspace = r.Header["X-Workspace-Id"]
		_, hadAuth = r.Header["Authorization"]
		_ = json.NewEncoder(w).Encode(map[string]any{"imported": []map[string]any{}})
	}))
	defer srv.Close()

	w := &Writer{Endpoint: srv.URL, HTTP: srv.Client()} // no ProjectID, no AuthHeader
	if _, err := w.WriteSession(SessionDerivation{
		Candidates: []Candidate{{Kind: KindDecision, LocalID: "d1", Text: "x"}},
	}); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	if hadWorkspace {
		t.Error("X-Workspace-Id should be absent when ProjectID is empty")
	}
	if hadAuth {
		t.Error("Authorization should be absent when AuthHeader is empty")
	}
}
