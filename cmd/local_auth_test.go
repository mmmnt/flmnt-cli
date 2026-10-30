package cmd

import (
	"encoding/base64"
	"fmt"
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
	t.Setenv("QUORUM_TOKEN_URL", "")
	t.Setenv("FLMNT_TOKEN_URL", "https://oauth.example/token")
	t.Setenv("QUORUM_CLIENT_ID", "")
	t.Setenv("FLMNT_CLIENT_ID", "flmnt-cli")
}

/*
brief, gate, derive, record-* and workspace authenticated from the keychain `flmnt login` filled; on

	a loopback server the checkout's headers-helper was then asked first, and ran quorum's
	scripts/mcp-auth-header.sh. That script is gone (QRM-ADR-015 §6): the harness credential is the
	token `flmnt login --local` stores under the local server URL, read from the keychain like any
	login's.
*/
func TestBestEffortBearerUsesTheKeychainForALoopbackServer(t *testing.T) {
	token := freshJWT(t, "local-login")
	stubKeychain(t, token, nil)

	if got := bestEffortBearer(briefCmd, "http://localhost:8000/mcp"); got != "Bearer "+token {
		t.Fatalf("got %q, want the keychain token", got)
	}
}

/* The hosted path is untouched: the stored login is the credential, exactly as before. */
func TestBestEffortBearerUsesTheKeychainForAHostedServer(t *testing.T) {
	token := freshJWT(t, "hosted-login")
	stubKeychain(t, token, nil)

	if got := bestEffortBearer(briefCmd, "https://mcp.production.flmnt.ai/mcp"); got != "Bearer "+token {
		t.Fatalf("got %q, want the keychain token", got)
	}
}

/* A server with no stored token gets nothing — never a helper's answer, never a default. */
func TestBestEffortBearerIsEmptyWithoutAStoredToken(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)

	for _, server := range []string{"http://localhost:8000/mcp", "https://mcp.production.flmnt.ai/mcp"} {
		if got := bestEffortBearer(briefCmd, server); got != "" {
			t.Fatalf("%s: got %q, want empty", server, got)
		}
	}
}

func base64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

/*
On a loopback server the project fallback of last resort was the X-Workspace-Id the checkout's

	headers-helper printed. The helper is retired: the chain is the same on every server — the flag,
	the repo's pin, the active workspace — and the MCP scopes a local session from the token's
	identity when nothing names a workspace.
*/
func TestResolveProjectFallsBackToTheActiveWorkspaceOnALoopbackServerToo(t *testing.T) {
	origCfg := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ActiveWorkspaceID: "729c1a1d"}, nil
	}
	t.Cleanup(func() { authHeaderLoadConfig = origCfg })

	for _, server := range []string{"http://localhost:8000/mcp", "https://mcp.production.flmnt.ai/mcp"} {
		t.Setenv("QUORUM_SERVER_URL", "")
		t.Setenv("FLMNT_SERVER_URL", server)
		if got := resolveProject(briefCmd, t.TempDir()); got != "729c1a1d" {
			t.Fatalf("%s: got %q, want the active workspace", server, got)
		}
	}
}
