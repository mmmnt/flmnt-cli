package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

// refusedServerURL names a loopback port nothing listens on, the way a stopped flmnt refuses.
func refusedServerURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr + "/mcp"
}

// isolateHookEnv keeps a hook test off this machine's login, keychain and checkout config.
func isolateHookEnv(t *testing.T) {
	t.Helper()
	origCfg, origTok := authHeaderLoadConfig, authHeaderLoadToken
	t.Cleanup(func() { authHeaderLoadConfig, authHeaderLoadToken = origCfg, origTok })
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	authHeaderLoadToken = func(string) (auth.TokenSet, error) { return auth.TokenSet{}, errors.New("no token") }
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Setenv("QUORUM_PROJECT_ID", "")
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", "")
	t.Chdir(t.TempDir())
}

func hookTestCmd(stdin string, bools ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	c := &cobra.Command{}
	for _, f := range []string{"server-url", "remote-url", "project", "graphql-endpoint", "workspace", "auth-cmd", "token-url", "client-id"} {
		c.Flags().String(f, "", "")
	}
	c.Flags().String("out", "training", "")
	c.Flags().Duration("threshold", 0, "")
	for _, f := range []string{"hook", "writer-dry-run"} {
		c.Flags().Bool(f, false, "")
	}
	for _, f := range bools {
		_ = c.Flags().Set(f, "true")
	}
	var stdout, stderr bytes.Buffer
	c.SetOut(&stdout)
	c.SetErr(&stderr)
	c.SetIn(strings.NewReader(stdin))
	return c, &stdout, &stderr
}

func assertNotConnected(t *testing.T, err error, stdout, stderr *bytes.Buffer) {
	t.Helper()
	if err != nil {
		t.Errorf("a hook must exit cleanly when no flmnt answers; got %v", err)
	}
	if !strings.Contains(stderr.String(), "FLMNT_NOT_CONNECTED") {
		t.Errorf("stderr must say FLMNT_NOT_CONNECTED; got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must stay empty; got %q", stdout.String())
	}
}

func TestGateSaysNotConnectedOnStderrAndExitsCleanlyWhenNoFlmntAnswers(t *testing.T) {
	isolateHookEnv(t)
	c, stdout, stderr := hookTestCmd(`{"session_id":"sess-1"}`)
	_ = c.Flags().Set("server-url", refusedServerURL(t))
	_ = c.Flags().Set("project", "ws-1")
	assertNotConnected(t, runGate(c, nil), stdout, stderr)
}

func TestDeriveHookSaysNotConnectedOnStderrAndExitsCleanlyWhenNoFlmntAnswers(t *testing.T) {
	isolateHookEnv(t)
	transcript := filepath.Join(t.TempDir(), "sess-1.jsonl")
	line := `{"type":"user","sessionId":"sess-1","cwd":"/repo","message":{"role":"user","content":"fix the gate"}}`
	if err := os.WriteFile(transcript, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"session_id": "sess-1", "transcript_path": transcript, "cwd": t.TempDir()})
	c, stdout, stderr := hookTestCmd(string(payload), "hook")
	_ = c.Flags().Set("server-url", refusedServerURL(t))
	_ = c.Flags().Set("project", "ws-1")
	assertNotConnected(t, runDerive(c, nil), stdout, stderr)
}

func TestCorpusHookSaysNotConnectedOnStderrAndExitsCleanlyWhenNoFlmntAnswers(t *testing.T) {
	isolateHookEnv(t)
	c, stdout, stderr := hookTestCmd("", "hook")
	_ = c.Flags().Set("server-url", refusedServerURL(t))
	_ = c.Flags().Set("project", "ws-1")
	assertNotConnected(t, runCorpus(c, nil), stdout, stderr)
}

func TestGateReadsTheKeyframeStreamOfTheSessionNamedOnStdin(t *testing.T) {
	isolateHookEnv(t)
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
	c, stdout, _ := hookTestCmd(`{"session_id":"sess-1","hook_event_name":"UserPromptSubmit"}`)
	_ = c.Flags().Set("server-url", srv.URL)
	_ = c.Flags().Set("graphql-endpoint", srv.URL)
	_ = c.Flags().Set("project", "ws-1")
	if err := runGate(c, nil); err != nil {
		t.Fatal(err)
	}
	if asked != "ws-1::sess-1::keyframe" {
		t.Fatalf("want ws-1::sess-1::keyframe, got %q", asked)
	}
	if !strings.Contains(stdout.String(), "[flmnt reminder]") {
		t.Fatalf("no keyframe on the session stream prints the reminder; got %q", stdout.String())
	}
}

func TestGateWithNoSessionOnStdinReadsNothingAndPrintsNothing(t *testing.T) {
	isolateHookEnv(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"data":{"memoryKeyframe":null}}`))
	}))
	defer srv.Close()
	c, stdout, stderr := hookTestCmd(`{}`)
	_ = c.Flags().Set("server-url", srv.URL)
	_ = c.Flags().Set("graphql-endpoint", srv.URL)
	_ = c.Flags().Set("project", "ws-1")
	if err := runGate(c, nil); err != nil {
		t.Fatal(err)
	}
	if hits != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("without a session there is no stream to read; hits=%d stdout=%q stderr=%q", hits, stdout.String(), stderr.String())
	}
}
