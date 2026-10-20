package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func runWhoami(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"whoami"}, args...))
	t.Cleanup(func() {
		_ = whoamiCmd.Flags().Set("json", "false")
		_ = whoamiCmd.Flags().Set("server-url", "")
	})
	err = rootCmd.Execute()
	return out.String(), errb.String(), err
}

func idToken(t *testing.T, claims string) string {
	t.Helper()
	return "h." + strings.TrimRight(base64url([]byte(claims)), "=") + ".s"
}

/*
The harness mints as the person logged in to flmnt and nobody else (founder ruling 2026-10-19). The

	helper asks the CLI who that is with exactly `flmnt whoami --json`: the stored login's claims,
	decoded offline from the keychain id token, as one JSON object — no network, no server resolution
	beyond the login itself, so it answers the same under the local hook wrapper and in a shell.
*/
func TestWhoamiJSONPrintsTheStoredLoginsClaims(t *testing.T) {
	stubKeychain(t, "", nil)
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: "acc", IDToken: idToken(t, `{"sub":"u-123","email":"mike@flmnt.ai","cognito:username":"mike"}`)}, nil
	}
	origCfg := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp", ActiveWorkspaceID: "729c1a1d"}, nil
	}
	t.Cleanup(func() { authHeaderLoadConfig = origCfg })
	t.Chdir(t.TempDir())

	stdout, stderr, err := runWhoami(t, "--json")
	if err != nil || stderr != "" {
		t.Fatalf("err %v stderr %q", err, stderr)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %q", stdout)
	}
	want := map[string]string{"sub": "u-123", "email": "mike@flmnt.ai", "username": "mike", "workspace": "729c1a1d"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (full: %v)", k, got[k], v, got)
		}
	}
}

/* The workspace is the one whoami reports: the repo's pin when there is one, as an id. */
func TestWhoamiJSONReportsTheRepoPinAsTheWorkspace(t *testing.T) {
	stubKeychain(t, "", nil)
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: idToken(t, `{"sub":"u-123"}`)}, nil
	}
	origCfg := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp", ActiveWorkspaceID: "729c1a1d"}, nil
	}
	t.Cleanup(func() { authHeaderLoadConfig = origCfg })
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.quorum.json", []byte(`{"server_url":"https://mcp.production.flmnt.ai","project_id":"bc674142"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	stdout, _, err := runWhoami(t, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	_ = json.Unmarshal([]byte(stdout), &got)
	if got["workspace"] != "bc674142" || got["sub"] != "u-123" {
		t.Fatalf("got %v", got)
	}
}

/* Not logged in: nothing on stdout, one line on stderr, exit non-zero — the helper then BLOCKS, by design. */
func TestWhoamiJSONRefusesWhenNotLoggedIn(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)
	origCfg := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp"}, nil
	}
	t.Cleanup(func() { authHeaderLoadConfig = origCfg })
	t.Chdir(t.TempDir())

	stdout, stderr, err := runWhoami(t, "--json")
	if err == nil {
		t.Fatal("expected a non-zero exit")
	}
	if stdout != "" {
		t.Errorf("stdout must be empty, got %q", stdout)
	}
	if n := strings.Count(strings.TrimSpace(stderr), "\n") + 1; n != 1 || !strings.Contains(stderr, "not logged in") {
		t.Errorf("want one reason line on stderr, got %q", stderr)
	}
}
