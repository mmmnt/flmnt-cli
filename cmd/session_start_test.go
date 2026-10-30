package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sessionFlmnt is a router that holds no stream, creates one on request, and records every call.
func sessionFlmnt(t *testing.T) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var calls []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		calls = append(calls, req.Variables)
		if strings.Contains(req.Query, "createMemoryStream") {
			id := req.Variables["projectId"].(string) + "::" + req.Variables["sessionId"].(string) + "::keyframe"
			_, _ = w.Write([]byte(`{"data":{"createMemoryStream":{"streamId":"` + id + `"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"memoryStream":null}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestSessionStartCreatesTheSessionKeyframeStreamAndPrintsItsID(t *testing.T) {
	isolateHookEnv(t)
	srv, calls := sessionFlmnt(t)
	c, stdout, stderr := hookTestCmd(`{"session_id":"sess-1","hook_event_name":"SessionStart"}`)
	_ = c.Flags().Set("server-url", srv.URL)
	_ = c.Flags().Set("graphql-endpoint", srv.URL)
	_ = c.Flags().Set("project", "ws-1")
	if err := runSessionStart(c, nil); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || (*calls)[1]["sessionId"] != "sess-1" || (*calls)[1]["type"] != "keyframe" {
		t.Fatalf("want a read then a keyframe create for sess-1; got %v", *calls)
	}
	if !strings.Contains(stdout.String(), `stream_id "ws-1::sess-1::keyframe"`) {
		t.Fatalf("stdout must name the stream to pass; got %q (stderr %q)", stdout.String(), stderr.String())
	}
}

func TestSessionStartSaysNotConnectedOnStderrAndExitsCleanlyWhenNoFlmntAnswers(t *testing.T) {
	isolateHookEnv(t)
	c, stdout, stderr := hookTestCmd(`{"session_id":"sess-1"}`)
	_ = c.Flags().Set("server-url", refusedServerURL(t))
	_ = c.Flags().Set("project", "ws-1")
	assertNotConnected(t, runSessionStart(c, nil), stdout, stderr)
}

func TestSessionStartWithNoSessionOnStdinAsksNothingAndPrintsNothing(t *testing.T) {
	isolateHookEnv(t)
	srv, calls := sessionFlmnt(t)
	c, stdout, stderr := hookTestCmd(`{}`)
	_ = c.Flags().Set("server-url", srv.URL)
	_ = c.Flags().Set("graphql-endpoint", srv.URL)
	_ = c.Flags().Set("project", "ws-1")
	if err := runSessionStart(c, nil); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("calls=%v stdout=%q stderr=%q", *calls, stdout.String(), stderr.String())
	}
}

func TestSessionStartPrintsNothingAndExitsCleanlyWhenTheServerRefuses(t *testing.T) {
	isolateHookEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer srv.Close()
	c, stdout, _ := hookTestCmd(`{"session_id":"sess-1"}`)
	_ = c.Flags().Set("server-url", srv.URL)
	_ = c.Flags().Set("graphql-endpoint", srv.URL)
	_ = c.Flags().Set("project", "ws-1")
	if err := runSessionStart(c, nil); err != nil {
		t.Fatalf("a session-start hook never fails a session; got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("a stream that could not be made sure of is not named; got %q", stdout.String())
	}
}

func TestSessionStartResolvesTheProjectFromTheCheckoutTheHookNames(t *testing.T) {
	isolateHookEnv(t)
	srv, calls := sessionFlmnt(t)
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".flmnt.json"), []byte(`{"server_url":"`+srv.URL+`","project_id":"ws-repo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"session_id": "sess-1", "cwd": repo})
	c, stdout, _ := hookTestCmd(string(payload))
	_ = c.Flags().Set("graphql-endpoint", srv.URL)
	if err := runSessionStart(c, nil); err != nil {
		t.Fatal(err)
	}
	if len(*calls) == 0 || (*calls)[0]["projectId"] != "ws-repo" {
		t.Fatalf("the project comes from the hook cwd's .flmnt.json; calls=%v", *calls)
	}
	if !strings.Contains(stdout.String(), "ws-repo::sess-1::keyframe") {
		t.Fatalf("got %q", stdout.String())
	}
}
