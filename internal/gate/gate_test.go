package gate

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

func gqlServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}

func TestRunReturnsReminderWhenNoKeyframe(t *testing.T) {
	srv := gqlServer(`{"data":{"memoryKeyframe":null}}`)
	defer srv.Close()
	out, _ := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "p", Threshold: 30 * time.Minute})
	if out != reminder("p::::keyframe") {
		t.Fatalf("want reminder, got %q", out)
	}
}

func TestRunSilentWhenKeyframeFresh(t *testing.T) {
	fresh := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	srv := gqlServer(fmt.Sprintf(`{"data":{"memoryKeyframe":{"createdAt":%q}}}`, fresh))
	defer srv.Close()
	out, _ := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "p", Threshold: 30 * time.Minute})
	if out != "" {
		t.Fatalf("want silent, got %q", out)
	}
}

func TestRunReturnsReminderWhenKeyframeStale(t *testing.T) {
	stale := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	srv := gqlServer(fmt.Sprintf(`{"data":{"memoryKeyframe":{"createdAt":%q}}}`, stale))
	defer srv.Close()
	out, _ := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "p", Threshold: 30 * time.Minute})
	if out != reminder("p::::keyframe") {
		t.Fatalf("want reminder, got %q", out)
	}
}

func TestRunReportsUnreachableWhenNoFlmntAnswers(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	out, err := Run(Config{GQL: apiclient.New("http://"+addr, "tok"), ProjectID: "p", SessionID: "s", Threshold: 30 * time.Minute})
	if out != "" || !apiclient.IsUnreachable(err) {
		t.Fatalf("want no reminder and an unreachable error, got out=%q err=%v", out, err)
	}
}

func TestRunReadsTheSessionKeyframeStream(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]string `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		asked = req.Variables["s"]
		_, _ = w.Write([]byte(`{"data":{"memoryKeyframe":null}}`))
	}))
	defer srv.Close()
	if _, err := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "ws-1", SessionID: "sess-1", Threshold: 30 * time.Minute}); err != nil {
		t.Fatal(err)
	}
	if asked != "ws-1::sess-1::keyframe" {
		t.Fatalf("want the session keyframe stream ws-1::sess-1::keyframe, got %q", asked)
	}
}

func TestReminderTextNamesFlmntAndNotQuorum(t *testing.T) {
	text := reminder("p::s::keyframe")
	if strings.Contains(strings.ToLower(text), "quorum") {
		t.Fatalf("the reminder must not name quorum: %q", text)
	}
	if !strings.HasPrefix(text, "[flmnt reminder]") {
		t.Fatalf("the reminder must open with [flmnt reminder]: %q", text)
	}
}

func TestTheReminderNamesTheSessionsExactKeyframeStream(t *testing.T) {
	srv := gqlServer(`{"data":{"memoryKeyframe":null}}`)
	defer srv.Close()
	out, _ := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "ws-1", SessionID: "sess-1", Threshold: 30 * time.Minute})
	if !strings.Contains(out, `"ws-1::sess-1::keyframe"`) {
		t.Fatalf("the reminder must name the session's keyframe stream ws-1::sess-1::keyframe: %q", out)
	}
	if strings.Contains(out, "list_streams") {
		t.Fatalf("the reminder must not send the agent to list_streams for a stream it can name: %q", out)
	}
}
