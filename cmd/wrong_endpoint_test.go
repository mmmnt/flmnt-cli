package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// wrongEndpoint is a server that answers every request, but not as flmnt's GraphQL router would: the
// way http://localhost:8000/mcp/graphql answers when a checkout declares no local_graphql_endpoint.
func wrongEndpoint(t *testing.T, status int, contentType, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var wrongAnswers = []struct {
	name        string
	status      int
	contentType string
	body        string
	want        string // what the line must name of the error
}{
	{"404", http.StatusNotFound, "text/plain", "404 page not found", "HTTP 404"},
	{"500", http.StatusInternalServerError, "text/plain", "internal error", "HTTP 500"},
	{"html", http.StatusOK, "text/html", "<!doctype html><html><body>flmnt</body></html>", "not a GraphQL response"},
}

// assertWrongEndpoint: the hook exits cleanly, prints nothing on stdout, and says FLMNT_NOT_CONNECTED on
// stderr naming the endpoint it called, the error, and how to declare the right one.
func assertWrongEndpoint(t *testing.T, err error, stdout, stderr *bytes.Buffer, endpoint, want string) {
	t.Helper()
	if err != nil {
		t.Errorf("a hook must exit cleanly when its endpoint answers with an error; got %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must stay empty; got %q", stdout.String())
	}
	line := stderr.String()
	for _, s := range []string{"FLMNT_NOT_CONNECTED", endpoint, want, "local_graphql_endpoint", "--server-url", "FLMNT_SERVER_URL"} {
		if !strings.Contains(line, s) {
			t.Errorf("stderr must name %q; got %q", s, line)
		}
	}
	if strings.Count(strings.TrimSpace(line), "\n") != 0 {
		t.Errorf("stderr must be one line; got %q", line)
	}
}

func wrongEndpointHook(t *testing.T, srv *httptest.Server, stdin string, bools ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	isolateHookEnv(t)
	c, stdout, stderr := hookTestCmd(stdin, bools...)
	_ = c.Flags().Set("server-url", srv.URL)
	_ = c.Flags().Set("graphql-endpoint", srv.URL+"/mcp/graphql")
	_ = c.Flags().Set("project", "ws-1")
	return c, stdout, stderr
}

func TestGateSaysNotConnectedNamingTheEndpointAndTheErrorWhenItAnswersWrongly(t *testing.T) {
	for _, tc := range wrongAnswers {
		t.Run(tc.name, func(t *testing.T) {
			srv := wrongEndpoint(t, tc.status, tc.contentType, tc.body)
			c, stdout, stderr := wrongEndpointHook(t, srv, `{"session_id":"sess-1"}`)
			assertWrongEndpoint(t, runGate(c, nil), stdout, stderr, srv.URL+"/mcp/graphql", tc.want)
		})
	}
}

func TestSessionStartSaysNotConnectedNamingTheEndpointAndTheErrorWhenItAnswersWrongly(t *testing.T) {
	for _, tc := range wrongAnswers {
		t.Run(tc.name, func(t *testing.T) {
			srv := wrongEndpoint(t, tc.status, tc.contentType, tc.body)
			c, stdout, stderr := wrongEndpointHook(t, srv, `{"session_id":"sess-1"}`)
			assertWrongEndpoint(t, runSessionStart(c, nil), stdout, stderr, srv.URL+"/mcp/graphql", tc.want)
		})
	}
}

func TestDeriveHookSaysNotConnectedNamingTheEndpointAndTheErrorWhenItAnswersWrongly(t *testing.T) {
	for _, tc := range wrongAnswers {
		t.Run(tc.name, func(t *testing.T) {
			srv := wrongEndpoint(t, tc.status, tc.contentType, tc.body)
			transcript := filepath.Join(t.TempDir(), "sess-1.jsonl")
			line := `{"type":"user","sessionId":"sess-1","cwd":"/repo","message":{"role":"user","content":"fix the gate"}}`
			if err := os.WriteFile(transcript, []byte(line+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(map[string]string{"session_id": "sess-1", "transcript_path": transcript, "cwd": t.TempDir()})
			c, stdout, stderr := wrongEndpointHook(t, srv, string(payload), "hook")
			assertWrongEndpoint(t, runDerive(c, nil), stdout, stderr, srv.URL+"/mcp/graphql", tc.want)
		})
	}
}

func TestCorpusHookSaysNotConnectedNamingTheEndpointAndTheErrorWhenItAnswersWrongly(t *testing.T) {
	for _, tc := range wrongAnswers {
		t.Run(tc.name, func(t *testing.T) {
			srv := wrongEndpoint(t, tc.status, tc.contentType, tc.body)
			c, stdout, stderr := wrongEndpointHook(t, srv, "", "hook")
			assertWrongEndpoint(t, runCorpus(c, nil), stdout, stderr, srv.URL+"/mcp/graphql", tc.want)
		})
	}
}

// A stream that does not exist is an answer, not a wrong endpoint: the gate keeps its reminder, and
// nothing is said on stderr.
func TestGateKeepsItsReminderAndSaysNothingOnStderrForAStreamThatDoesNotExist(t *testing.T) {
	srv := wrongEndpoint(t, http.StatusOK, "application/json", `{"data":{"memoryKeyframe":null}}`)
	c, stdout, stderr := wrongEndpointHook(t, srv, `{"session_id":"sess-1"}`)
	if err := runGate(c, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "[flmnt reminder]") || stderr.Len() != 0 {
		t.Fatalf("want the reminder and a quiet stderr; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// A domain refusal in a GraphQL envelope (unauthorized, a stream refused) is the router answering:
// every hook stays as quiet as it was.
func TestHooksStayQuietOnAGraphQLRefusal(t *testing.T) {
	srv := wrongEndpoint(t, http.StatusOK, "application/json", `{"data":null,"errors":[{"message":"unauthorized"}]}`)
	for name, run := range map[string]func(*cobra.Command, []string) error{"gate": runGate, "session-start": runSessionStart} {
		t.Run(name, func(t *testing.T) {
			c, stdout, stderr := wrongEndpointHook(t, srv, `{"session_id":"sess-1"}`)
			if err := run(c, nil); err != nil {
				t.Fatal(err)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("a GraphQL refusal stays quiet; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}
