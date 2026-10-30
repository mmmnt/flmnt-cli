package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
)

func runWhoami(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"whoami"}, args...))
	reset := func() {
		_ = whoamiCmd.Flags().Set("server-url", "")
		_ = whoamiCmd.Flags().Set("json", "false")
		_ = whoamiCmd.Flags().Set("client", "")
	}
	reset()
	t.Cleanup(reset)
	err = rootCmd.Execute()
	return out.String(), errb.String(), err
}

func idToken(t *testing.T, claims string) string {
	t.Helper()
	return "h." + base64url([]byte(claims)) + ".s"
}

var localMike = identity.Derive("ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", "mike")

func stubIdentity(t *testing.T, id identity.Identity, err error) {
	t.Helper()
	orig := identityLoad
	identityLoad = func() (identity.Identity, error) { return id, err }
	t.Cleanup(func() { identityLoad = orig })
}

func stubConfig(t *testing.T, cfg auth.CLIConfig) {
	t.Helper()
	orig := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return cfg, nil }
	t.Cleanup(func() { authHeaderLoadConfig = orig })
}

func decodeWhoami(t *testing.T, stdout string) map[string]string {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %q", stdout)
	}
	return got
}

/*
The local helper learns who to mint for by asking exactly `flmnt whoami --json --client <tool>`: on

	a loopback server the answer is the LOCAL LOGIN — the derived sub and the client id derived for
	that tool — and never anything from the hosted login.
*/
func TestWhoamiJSONAnswersFromTheLocalLoginOnALoopbackServer(t *testing.T) {
	stubIdentity(t, localMike, nil)
	stubKeychain(t, idToken(t, `{"sub":"hosted-sub","email":"other@flmnt.ai","cognito:username":"hosted"}`), nil)
	stubConfig(t, auth.CLIConfig{ActiveWorkspaceID: "729c1a1d", ClientID: "flmnt-cli"})
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.quorum.json", []byte(`{"server_url":"https://mcp.production.flmnt.ai","project_id":"bc674142"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	stdout, stderr, err := runWhoami(t, "--json", "--client", "flmnt-cli", "--server-url", "http://localhost:8000/mcp")
	if err != nil || stderr != "" {
		t.Fatalf("err %v stderr %q", err, stderr)
	}
	got := decodeWhoami(t, stdout)
	want := map[string]string{"sub": localMike.Sub, "workspace": "bc674142", "client_id": localMike.ClientID("flmnt-cli")}
	if len(got) != len(want) {
		t.Errorf("exactly {sub,workspace,client_id}; got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

/* On a hosted server the source is the stored login, and the client id is the registered OAuth client. */
func TestWhoamiJSONAnswersFromTheKeychainOnAHostedServer(t *testing.T) {
	stubIdentity(t, localMike, nil)
	stubKeychain(t, "", nil)
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: "acc", IDToken: idToken(t, `{"sub":"u-123","email":"mike@flmnt.ai","cognito:username":"mike"}`)}, nil
	}
	stubConfig(t, auth.CLIConfig{ActiveWorkspaceID: "729c1a1d", ClientID: "flmnt-cli"})
	t.Chdir(t.TempDir())

	stdout, _, err := runWhoami(t, "--json", "--client", "flmnt-cli", "--server-url", "https://mcp.production.flmnt.ai/mcp")
	if err != nil {
		t.Fatal(err)
	}
	got := decodeWhoami(t, stdout)
	want := map[string]string{"sub": "u-123", "workspace": "729c1a1d", "client_id": "flmnt-cli"}
	if len(got) != len(want) {
		t.Errorf("no email, no username; got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}

	stdout, _, _ = runWhoami(t, "--json", "--server-url", "https://mcp.production.flmnt.ai/mcp")
	if _, present := decodeWhoami(t, stdout)["client_id"]; present {
		t.Error("client_id is printed only when --client names a tool")
	}
}

/* Not logged in, either kind: nothing on stdout, one reason line on stderr, exit non-zero — the helper then blocks, by design. */
func TestWhoamiJSONRefusesWhenThereIsNoLogin(t *testing.T) {
	stubIdentity(t, identity.Identity{}, identity.ErrNotFound)
	stubKeychain(t, "", auth.ErrNotFound)
	stubConfig(t, auth.CLIConfig{})
	t.Chdir(t.TempDir())

	for _, server := range []string{"http://localhost:8000/mcp", "https://mcp.production.flmnt.ai/mcp"} {
		stdout, stderr, err := runWhoami(t, "--json", "--server-url", server)
		if err == nil {
			t.Fatalf("%s: expected a non-zero exit", server)
		}
		if stdout != "" {
			t.Errorf("%s: stdout must be empty, got %q", server, stdout)
		}
		if n := strings.Count(strings.TrimSpace(stderr), "\n") + 1; n != 1 || !strings.Contains(stderr, "log") {
			t.Errorf("%s: want one reason line on stderr, got %q", server, stderr)
		}
	}
}

/* Plain whoami keeps its one line for both sources; locally the identity is "local identity <sub>". */
func TestWhoamiNamesTheLocalLoginOnALoopbackServer(t *testing.T) {
	stubIdentity(t, localMike, nil)
	stubKeychain(t, "", auth.ErrNotFound)
	stubConfig(t, auth.CLIConfig{ActiveWorkspaceName: "sift"})
	t.Chdir(t.TempDir())

	stdout, _, err := runWhoami(t, "--server-url", "http://localhost:8000/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "local identity "+localMike.Sub) || !strings.Contains(stdout, "sift") {
		t.Fatalf("got %q", stdout)
	}

	stubIdentity(t, identity.Identity{}, identity.ErrNotFound)
	stdout, _, _ = runWhoami(t, "--server-url", "http://localhost:8000/mcp")
	if !strings.Contains(stdout, "Not logged in") || !strings.Contains(stdout, "flmnt login --local") {
		t.Fatalf("got %q", stdout)
	}
}

/*
The bug as reported: `flmnt login --local`, then `flmnt whoami` in a repo pinned to production
printed the email of the production token still in the keychain. The login is the session now: the
recorded local login answers, with no --server-url and whatever the repo declares, and the hosted
token is never consulted.
*/
func TestWhoamiAnswersFromTheRecordedLocalLoginInARepoPinnedToProduction(t *testing.T) {
	stubIdentity(t, localMike, nil)
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		t.Error("the hosted token must not be consulted while the local login is the session")
		return auth.TokenSet{}, nil
	}
	t.Cleanup(func() { authHeaderLoadToken = auth.LoadToken })
	stubConfig(t, auth.CLIConfig{Login: auth.LoginLocal, ServerURL: defaultLocalURL, Sub: localMike.Sub})
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.quorum.json", []byte(`{"server_url":"https://mcp.production.flmnt.ai","project_id":"bc674142","project_name":"quorum"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	stdout, _, err := runWhoami(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "local identity "+localMike.Sub) || strings.Contains(stdout, "@") {
		t.Fatalf("got %q", stdout)
	}
	stdout, _, err = runWhoami(t, "--json", "--client", "flmnt-cli")
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeWhoami(t, stdout); got["sub"] != localMike.Sub || got["workspace"] != "bc674142" {
		t.Fatalf("got %v", got)
	}
}

/* Nothing logged in and no server named: a plain reason, not a demand for a flag. */
func TestWhoamiSaysNotLoggedInWhenThereIsNoSessionAndNoServer(t *testing.T) {
	stubIdentity(t, identity.Identity{}, identity.ErrNotFound)
	stubKeychain(t, "", auth.ErrNotFound)
	stubConfig(t, auth.CLIConfig{})
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())

	stdout, _, err := runWhoami(t)
	if err != nil || !strings.Contains(stdout, "Not logged in") || !strings.Contains(stdout, "flmnt login") {
		t.Fatalf("err %v stdout %q", err, stdout)
	}
	if _, stderr, err := runWhoami(t, "--json"); err == nil || !strings.Contains(stderr, "not logged in") {
		t.Fatalf("--json refuses with the reason: err %v stderr %q", err, stderr)
	}
}

/*
The mirror of the bug, under the ruling that identity and session are separate (2026-10-20): a
hosted session on a machine that still holds its local identity — as every machine does after
`flmnt login` now — answers from the hosted token, even in a repo pinned to the local harness. The
session's kind decides, never which credential happens to exist.
*/
func TestWhoamiAnswersFromTheHostedSessionWhileTheLocalIdentityIsPresent(t *testing.T) {
	stubIdentity(t, localMike, nil)
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: "acc", IDToken: idToken(t, `{"sub":"u-123","email":"mike@flmnt.ai"}`)}, nil
	}
	t.Cleanup(func() { authHeaderLoadToken = auth.LoadToken })
	stubConfig(t, auth.CLIConfig{Login: auth.LoginHosted, ServerURL: "https://mcp.production.flmnt.ai", Sub: "u-123", ClientID: "flmnt-cli"})
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.quorum.json", []byte(`{"server_url":"http://localhost:8000/mcp","project_id":"bc674142","project_name":"quorum"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	stdout, _, err := runWhoami(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "mike@flmnt.ai") || strings.Contains(stdout, localMike.Sub) {
		t.Fatalf("got %q", stdout)
	}
	stdout, _, err = runWhoami(t, "--json", "--client", "flmnt-cli")
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeWhoami(t, stdout); got["sub"] != "u-123" || got["client_id"] != "flmnt-cli" {
		t.Fatalf("got %v", got)
	}
}

/* whoami names the subs this machine formerly answered as, so the history is in plain sight. */
func TestWhoamiNamesTheFormerSubs(t *testing.T) {
	former := identity.Derive("ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", "mike")
	current := identity.Remint(former, "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", "mike", time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC))
	stubIdentity(t, current, nil)
	stubConfig(t, auth.CLIConfig{Login: auth.LoginLocal, ServerURL: defaultLocalURL, Sub: current.Sub})
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())

	stdout, _, err := runWhoami(t)
	if err != nil || !strings.Contains(stdout, "local identity "+current.Sub+" (formerly "+former.Sub+")") {
		t.Fatalf("err %v stdout %q", err, stdout)
	}
}
