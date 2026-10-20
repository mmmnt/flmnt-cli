package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func freshJWT(t *testing.T, sub string) string {
	t.Helper()
	claims := fmt.Sprintf(`{"sub":%q,"exp":%d}`, sub, time.Now().Add(time.Hour).Unix())
	return "h." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".s"
}

func stubKeychain(t *testing.T, token string, err error) {
	t.Helper()
	orig := authHeaderLoadToken
	authHeaderLoadToken = func(string) (auth.TokenSet, error) { return auth.TokenSet{AccessToken: token}, err }
	t.Cleanup(func() { authHeaderLoadToken = orig })
	t.Setenv("QUORUM_TOKEN_URL", "https://oauth.example/token")
	t.Setenv("QUORUM_CLIENT_ID", "flmnt-cli")
}

func stubHelper(t *testing.T, command string) {
	t.Helper()
	if err := rootCmd.PersistentFlags().Set("auth-cmd", command); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rootCmd.PersistentFlags().Set("auth-cmd", "") })
}

// helper wraps a fixture so the `--client flmnt-cli` the CLI appends is received, not echoed.
func helper(body string) string { return "f() { " + body + "; }; f" }

// A helper that is unique per test, so the per-command memo never bleeds between tests.
func helperPrinting(t *testing.T, bearer string) string {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "runs")
	return helper(fmt.Sprintf(`echo run >> %q; echo '{"Authorization":"Bearer %s","X-Workspace-Id":"ws-local"}'`, marker, bearer))
}

/*
brief, gate, derive, record-* and workspace authenticated from the keychain `flmnt login` filled; on

	a loopback server the checkout's headers-helper was then asked first, and ran quorum's
	scripts/mcp-auth-header.sh. That script is gone (QRM-ADR-015 §6): the harness credential is the
	token `flmnt login --local` stores under the local server URL, read from the keychain like any
	login's. A helper configured anywhere is never run.
*/
func TestBestEffortBearerUsesTheKeychainForALoopbackServer(t *testing.T) {
	token := freshJWT(t, "local-login")
	stubKeychain(t, token, nil)
	stubHelper(t, helperPrinting(t, "HELPER-TOKEN"))

	if got := bestEffortBearer(briefCmd, "http://localhost:8000/mcp"); got != "Bearer "+token {
		t.Fatalf("got %q, want the keychain token", got)
	}
}

/* The hosted path is untouched: the stored login is the credential, exactly as before. */
func TestBestEffortBearerUsesTheKeychainForAHostedServer(t *testing.T) {
	token := freshJWT(t, "hosted-login")
	stubKeychain(t, token, nil)
	stubHelper(t, helperPrinting(t, "HELPER-TOKEN"))

	if got := bestEffortBearer(briefCmd, "https://mcp.production.flmnt.ai/mcp"); got != "Bearer "+token {
		t.Fatalf("got %q, want the keychain token", got)
	}
}

/* A server with no stored token gets nothing — never a helper's answer, never a default. */
func TestBestEffortBearerIsEmptyWithoutAStoredToken(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)
	stubHelper(t, helperPrinting(t, "HELPER-TOKEN"))

	for _, server := range []string{"http://localhost:8000/mcp", "https://mcp.production.flmnt.ai/mcp"} {
		if got := bestEffortBearer(briefCmd, server); got != "" {
			t.Fatalf("%s: got %q, want empty", server, got)
		}
	}
}

/* The helper costs seconds (it mints); one command runs it once however many callers ask. */
func TestHelperOutputIsMemoizedPerCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runs")
	stubHelper(t, helper(fmt.Sprintf(`echo run >> %q; echo '{"Authorization":"Bearer T","X-Workspace-Id":"ws-local"}'`, marker)))

	for range 3 {
		if _, err := localCredentials(briefCmd, ""); err != nil {
			t.Fatal(err)
		}
	}

	runs, _ := os.ReadFile(marker)
	if n := strings.Count(string(runs), "run"); n != 1 {
		t.Fatalf("helper ran %d times, want 1", n)
	}
}

func TestLocalCredentialsCarryTheHelpersWorkspace(t *testing.T) {
	stubHelper(t, helperPrinting(t, "T"))

	h, err := localCredentials(briefCmd, "")
	if err != nil || h.Workspace != "ws-local" {
		t.Fatalf("got %+v err %v", h, err)
	}
}

func base64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

/*
The project fallback of last resort was the machine-wide ACTIVE workspace — a hosted setting that

	names nothing on the harness. On a loopback server the helper's X-Workspace-Id is the workspace the
	MCP scopes the session to, so it is the fallback there; the hosted chain is unchanged.
*/
func TestResolveProjectFallsBackToTheHelpersWorkspaceOnALoopbackServer(t *testing.T) {
	stubHelper(t, helperPrinting(t, "T"))
	origCfg := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ActiveWorkspaceID: "729c1a1d"}, nil
	}
	t.Cleanup(func() { authHeaderLoadConfig = origCfg })
	t.Setenv("QUORUM_SERVER_URL", "http://localhost:8000/mcp")

	if got := resolveProject(briefCmd, t.TempDir()); got != "ws-local" {
		t.Fatalf("got %q, want the helper's workspace", got)
	}

	t.Setenv("QUORUM_SERVER_URL", "https://mcp.production.flmnt.ai/mcp")
	if got := resolveProject(briefCmd, t.TempDir()); got != "729c1a1d" {
		t.Fatalf("got %q, want the active workspace for a hosted server", got)
	}
}
