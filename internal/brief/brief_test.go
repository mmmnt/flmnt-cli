package brief

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// TestRenderRunsAuthenticatedGraphQL exercises Render against the router GraphQL surface, asserting
// the bearer token is sent and the briefing surfaces the keyframe + typed entries per stream.
func TestRenderRunsAuthenticatedGraphQL(t *testing.T) {
	const ws = "ws-1"
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(b, &req)
		s, _ := req.Variables["s"].(string)
		switch {
		case strings.Contains(req.Query, "memoryKeyframe"):
			_, _ = w.Write([]byte(`{"data":{"memoryKeyframe":{"content":"Current understanding of the system."}}}`))
		case strings.Contains(req.Query, "memoryEntries") && s == ws+"::domain":
			_, _ = w.Write([]byte(`{"data":{"memoryEntries":[{"entryType":"commit.recorded","content":"fix: dashboard tail"},{"entryType":"decision.made","content":"Use memoryImport for writes."}]}}`))
		case strings.Contains(req.Query, "memoryEntries") && s == ws+"::mistake":
			_, _ = w.Write([]byte(`{"data":{"memoryEntries":[{"entryType":"decision.mistake","content":"Hardcoded core-url on a public CLI."}]}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{"memoryEntries":[]}}`))
		}
	}))
	defer srv.Close()

	out, err := Render(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: ws})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want Bearer tok", gotAuth)
	}
	for _, want := range []string{
		"Current understanding of the system.",
		"fix: dashboard tail",
		"Use memoryImport for writes.",
		"Hardcoded core-url on a public CLI.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("briefing missing %q:\n%s", want, out)
		}
	}
}

// TestRenderEmptyWhenNoMemory returns "" (nothing to inject) when every stream is empty.
func TestRenderEmptyWhenNoMemory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "memoryKeyframe") {
			_, _ = w.Write([]byte(`{"data":{"memoryKeyframe":null}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"memoryEntries":[]}}`))
	}))
	defer srv.Close()
	out, err := Render(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "ws-1"})
	if err != nil || out != "" {
		t.Fatalf("want empty briefing, got err=%v out=%q", err, out)
	}
}
