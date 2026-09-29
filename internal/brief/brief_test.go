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

/*
The briefing had no place for what the founder actually said. It carried commits, recorded decisions and
mistakes — all of them the agent's own output — so a session started knowing what it had done and not
what it had been asked. Captured prompts close that, but only usefully once they are separated: a
direction still outstanding and a question still unanswered need different things from the reader, and
the majority of captured sentences are neither.

The unit is the SENTENCE, because 41% of real messages carry more than one intent. "flmnt updated.
verify all commands updated/functional." belongs in directions for its second half, not whole.
*/
func TestRenderSeparatesDirectionsFromQuestionsInCapturedPrompts(t *testing.T) {
	const ws = "ws-1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "memoryKeyframe") {
			_, _ = w.Write([]byte(`{"data":{"memoryKeyframe":null}}`))
			return
		}
		if strings.Contains(string(b), ws+"::domain") {
			_, _ = w.Write([]byte(`{"data":{"memoryEntries":[{"entryType":"prompt.captured","content":"flmnt updated. verify all commands updated/functional."},{"entryType":"prompt.captured","content":"was that fix pushed?"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"memoryEntries":[]}}`))
	}))
	defer srv.Close()

	out, err := Render(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: ws})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	directions := section(out, "Their recent directions:")
	questions := section(out, "Their recent questions:")
	if !strings.Contains(directions, "verify all commands updated/functional.") {
		t.Errorf("directions missing the directive sentence:\n%s", out)
	}
	if strings.Contains(directions, "flmnt updated.") {
		t.Errorf("directions carried an assertion from the same message:\n%s", out)
	}
	if !strings.Contains(questions, "was that fix pushed?") {
		t.Errorf("questions missing the inquiry:\n%s", out)
	}
}

// section returns the text under a heading, up to the blank line that ends it.
func section(out, heading string) string {
	i := strings.Index(out, heading)
	if i < 0 {
		return ""
	}
	rest := out[i+len(heading):]
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}
