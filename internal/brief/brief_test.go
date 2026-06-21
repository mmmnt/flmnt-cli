package brief

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBaseURLStripsMCPSuffix(t *testing.T) {
	cases := map[string]string{
		"https://mcp.staging.flmnt.dev":      "https://mcp.staging.flmnt.dev",
		"https://mcp.staging.flmnt.dev/":     "https://mcp.staging.flmnt.dev",
		"https://mcp.staging.flmnt.dev/mcp":  "https://mcp.staging.flmnt.dev",
		"https://mcp.staging.flmnt.dev/mcp/": "https://mcp.staging.flmnt.dev",
		"https://mcp.staging.flmnt.dev/sse":  "https://mcp.staging.flmnt.dev",
	}
	for in, want := range cases {
		if got := (Config{Endpoint: in}).baseURL(); got != want {
			t.Errorf("baseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenderAgainstProxyShape exercises Render against the staging proxy's REST read surface
// (the /streams passthrough), asserting the workspace header is sent, the /mcp suffix is stripped,
// and the briefing surfaces the keyframe + typed entries.
func TestRenderAgainstProxyShape(t *testing.T) {
	const ws = "ws-1"
	var gotWorkspace string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotWorkspace = r.Header.Get("X-Workspace-Id")
		switch {
		case r.URL.Path == "/streams/"+ws+"::domain/keyframes/latest":
			_ = json.NewEncoder(w).Encode(keyframeResp{Content: "Current understanding of the system."})
		case r.URL.Path == "/streams/"+ws+"::domain/entries":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"entryType": "commit.recorded", "payload": map[string]any{"title": "fix: dashboard tail"}},
				{"entryType": "decision.made", "payload": map[string]any{"content": "Use /sync/import for writes."}},
			})
		case r.URL.Path == "/streams/"+ws+"::mistake/entries":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"entryType": "decision.mistake", "payload": map[string]any{"content": "Hardcoded core-url on a public CLI."}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Endpoint carries a /mcp suffix to prove normalization; ProjectID drives X-Workspace-Id.
	out, err := Render(Config{Endpoint: srv.URL + "/mcp", ProjectID: ws, AuthHeader: "Bearer x"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if gotWorkspace != ws {
		t.Errorf("X-Workspace-Id = %q, want %q", gotWorkspace, ws)
	}
	for _, want := range []string{
		"Current understanding of the system.",
		"fix: dashboard tail",
		"Use /sync/import for writes.",
		"Hardcoded core-url on a public CLI.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("briefing missing %q:\n%s", want, out)
		}
	}
}
