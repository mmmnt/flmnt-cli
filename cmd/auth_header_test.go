package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/pflag"
)

func makeTestJWT(exp int64) string {
	payload := fmt.Sprintf(`{"sub":"s","exp":%d}`, exp)
	return "h." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}

func runAuthHeaderArgs(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	authHeaderCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"mcp", "auth-header"}, args...))
	err = rootCmd.Execute()
	return out.String(), errb.String(), err
}

func TestAuthHeaderPrintsAuthorizationAndWorkspaceHeaders(t *testing.T) {
	orig := authHeaderLoadToken
	defer func() { authHeaderLoadToken = orig }()
	access := makeTestJWT(time.Now().Add(time.Hour).Unix())
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: access, RefreshToken: "r"}, nil
	}

	stdout, stderr, err := runAuthHeaderArgs(t,
		"--server-url", "https://mcp.example",
		"--token-url", "https://auth.example/oauth2/token",
		"--client-id", "client-1",
		"--workspace", "ws-123",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var hdr map[string]string
	if err := json.Unmarshal([]byte(stdout), &hdr); err != nil {
		t.Fatalf("stdout not JSON: %q (%v)", stdout, err)
	}
	if hdr["Authorization"] != "Bearer "+access {
		t.Fatalf("authorization: %q", hdr["Authorization"])
	}
	if hdr["X-Workspace-Id"] != "ws-123" {
		t.Fatalf("workspace: %q", hdr["X-Workspace-Id"])
	}
}

func TestAuthHeaderFailsClosedWhenNotLoggedIn(t *testing.T) {
	orig := authHeaderLoadToken
	defer func() { authHeaderLoadToken = orig }()
	authHeaderLoadToken = func(string) (auth.TokenSet, error) { return auth.TokenSet{}, auth.ErrNotFound }

	stdout, _, err := runAuthHeaderArgs(t,
		"--server-url", "https://mcp.example",
		"--token-url", "https://auth.example/oauth2/token",
		"--client-id", "client-1",
	)
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
}

func TestAuthHeaderDiscoversTokenEndpointAndRefreshes(t *testing.T) {
	origLoad := authHeaderLoadToken
	origStore := authHeaderStoreToken
	defer func() { authHeaderLoadToken = origLoad; authHeaderStoreToken = origStore }()

	var stored auth.TokenSet
	authHeaderStoreToken = func(_ string, ts auth.TokenSet) error { stored = ts; return nil }

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token_endpoint": srv.URL + "/oauth2/token"})
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "refreshed-access"})
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: makeTestJWT(time.Now().Add(10 * time.Second).Unix()), RefreshToken: "r"}, nil
	}

	stdout, stderr, err := runAuthHeaderArgs(t, "--server-url", srv.URL, "--client-id", "client-1", "--workspace", "ws")
	if err != nil {
		t.Fatalf("execute: %v (stderr %s)", err, stderr)
	}
	var hdr map[string]string
	if err := json.Unmarshal([]byte(stdout), &hdr); err != nil {
		t.Fatalf("stdout not JSON: %q", stdout)
	}
	if hdr["Authorization"] != "Bearer refreshed-access" {
		t.Fatalf("authorization: %q", hdr["Authorization"])
	}
	if stored.AccessToken != "refreshed-access" {
		t.Fatalf("refreshed token not persisted: %+v", stored)
	}
}

func TestAuthHeaderUsesActiveWorkspaceFromConfig(t *testing.T) {
	origT := authHeaderLoadToken
	origC := authHeaderLoadConfig
	defer func() { authHeaderLoadToken = origT; authHeaderLoadConfig = origC }()
	access := makeTestJWT(time.Now().Add(time.Hour).Unix())
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: access, RefreshToken: "r"}, nil
	}
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ActiveWorkspaceID: "ws-active"}, nil
	}

	stdout, _, err := runAuthHeaderArgs(t, "--server-url", "https://m", "--token-url", "https://t", "--client-id", "c")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var hdr map[string]string
	_ = json.Unmarshal([]byte(stdout), &hdr)
	if hdr["X-Workspace-Id"] != "ws-active" {
		t.Fatalf("workspace from config: %q", hdr["X-Workspace-Id"])
	}
}

func TestAuthHeaderOmitsWorkspaceWhenNoneActive(t *testing.T) {
	origT := authHeaderLoadToken
	origC := authHeaderLoadConfig
	defer func() { authHeaderLoadToken = origT; authHeaderLoadConfig = origC }()
	access := makeTestJWT(time.Now().Add(time.Hour).Unix())
	authHeaderLoadToken = func(string) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: access, RefreshToken: "r"}, nil
	}
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }

	stdout, _, err := runAuthHeaderArgs(t, "--server-url", "https://m", "--token-url", "https://t", "--client-id", "c")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var hdr map[string]string
	_ = json.Unmarshal([]byte(stdout), &hdr)
	if _, ok := hdr["X-Workspace-Id"]; ok {
		t.Fatalf("expected no workspace header, got %v", hdr)
	}
}
