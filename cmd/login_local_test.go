package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
)

// runLogin runs `flmnt login`; the caller stubs the seams (stubLoginSeams) first, since a login wipes
// the keychain and the local login, and a test must never reach the real ones.
func runLogin(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	t.Setenv("QUORUM_SERVER_URL", "")
	stdout, _, err = runLoginArgs(t, args...)
	return stdout, err
}

/*
A local flmnt mints for a declared person. The declaration is a LOCAL LOGIN the CLI owns, with no

	input: the marker the host machine already carries and the OS user, digested to a sub that is
	persisted — the file is what makes the login explicit. Founder ruling 2026-10-19.
*/
func TestLoginLocalDerivesAndRecordsTheSub(t *testing.T) {
	stubLoginSeams(t)
	origMarker, origUser, origSave := identityMarker, identityOSUser, identitySave
	identityMarker = func() (string, error) { return "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", nil }
	identityOSUser = func() (string, error) { return "mike", nil }
	var saved identity.Identity
	identitySave = func(i identity.Identity) error { saved = i; return nil }
	t.Cleanup(func() { identityMarker, identityOSUser, identitySave = origMarker, origUser, origSave })

	stdout, err := runLogin(t, "--local")
	if err != nil {
		t.Fatal(err)
	}
	want := identity.Derive("ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", "mike").Sub
	if saved.Sub != want || saved.CreatedAt == "" {
		t.Fatalf("saved %+v, want sub %s", saved, want)
	}
	if !strings.Contains(stdout, want) {
		t.Fatalf("the derived sub must be printed; got %q", stdout)
	}
}

/* Nothing is ever invented: a machine with no marker is a refusal and nothing is written. */
func TestLoginLocalRefusesWithoutAMarker(t *testing.T) {
	stubLoginSeams(t)
	origMarker, origUser, origSave := identityMarker, identityOSUser, identitySave
	identityMarker = func() (string, error) { return "", errors.New("no machine marker could be read from this host") }
	identityOSUser = func() (string, error) { return "mike", nil }
	saves := 0
	identitySave = func(identity.Identity) error { saves++; return nil }
	t.Cleanup(func() { identityMarker, identityOSUser, identitySave = origMarker, origUser, origSave })

	if _, err := runLogin(t, "--local"); err == nil || !strings.Contains(err.Error(), "marker") {
		t.Fatalf("without a marker: %v", err)
	}
	if saves != 0 {
		t.Fatal("a refused login must write nothing")
	}
}

/*
The local login IS the login from then on. The CLI holds one login at a time (founder ruling
2026-10-20): the session config records the local kind and the loopback server it is for, so no
command needs --server-url afterwards, and every hosted token is cleared so nothing hosted — an
email from an old production token — can answer for this person again.
*/
func TestLoginLocalBecomesTheSessionAndClearsTheHostedLogin(t *testing.T) {
	origMarker, origUser, origSave := identityMarker, identityOSUser, identitySave
	identityMarker = func() (string, error) { return "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", nil }
	identityOSUser = func() (string, error) { return "mike", nil }
	identitySave = func(identity.Identity) error { return nil }
	t.Cleanup(func() { identityMarker, identityOSUser, identitySave = origMarker, origUser, origSave })
	stubLoginSeams(t)
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }
	wiped := false
	loginDeleteAllTokens = func() ([]string, error) {
		wiped = true
		return []string{"https://mcp.production.flmnt.ai|access_token", "https://mcp.production.flmnt.ai|id_token"}, nil
	}

	stdout, err := runLogin(t, "--local")
	if err != nil {
		t.Fatal(err)
	}
	if !wiped {
		t.Error("a local login must clear every hosted token: one login at a time")
	}
	want := auth.CLIConfig{Login: auth.LoginLocal, ServerURL: defaultLocalURL, Sub: localMike.Sub, Identity: localMike.Sub, LoggedInAt: "2026-10-20T16:00:00Z"}
	if saved != want {
		t.Errorf("session recorded %+v, want %+v", saved, want)
	}
	if !strings.Contains(stdout, defaultLocalURL) || !strings.Contains(stdout, "mcp.production.flmnt.ai") {
		t.Errorf("the output must say which server the session is for and that the hosted login was cleared; got %q", stdout)
	}
}

/* The server a local login records is the loopback one named, and only a loopback one. */
func TestLoginLocalRecordsTheLoopbackServerNamedAndRefusesAHostedOne(t *testing.T) {
	origMarker, origUser, origSave := identityMarker, identityOSUser, identitySave
	identityMarker = func() (string, error) { return "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", nil }
	identityOSUser = func() (string, error) { return "mike", nil }
	saves := 0
	identitySave = func(identity.Identity) error { saves++; return nil }
	t.Cleanup(func() { identityMarker, identityOSUser, identitySave = origMarker, origUser, origSave })
	stubLoginSeams(t)
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }

	if _, err := runLogin(t, "--local", "--server-url", "http://127.0.0.1:9000/mcp"); err != nil {
		t.Fatal(err)
	}
	if saved.ServerURL != "http://127.0.0.1:9000/mcp" {
		t.Errorf("recorded %q", saved.ServerURL)
	}
	if _, err := runLogin(t, "--local", "--server-url", "https://mcp.production.flmnt.ai"); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("a hosted server is refused for a local login: %v", err)
	}
	if saves != 1 {
		t.Errorf("the refused login must write nothing; saves = %d", saves)
	}
}

/*
A hosted login replaces the session the same way: the local login goes, every token goes before
its own is stored, and the config records the hosted kind and who the token says we are.
*/
func TestLoginHostedBecomesTheSessionAndClearsTheLocalLogin(t *testing.T) {
	stubLoginSeams(t)
	origPKCE, origStore := loginRunPKCEFlow, loginStoreToken
	t.Cleanup(func() { loginRunPKCEFlow, loginStoreToken = origPKCE, origStore })
	loginRunPKCEFlow = func(auth.PKCEConfig, func(string) error) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: "acc", RefreshToken: "ref", IDToken: idToken(t, `{"sub":"u-123","email":"mike@flmnt.ai"}`)}, nil
	}
	order := []string{}
	loginStoreToken = func(string, auth.TokenSet) error { order = append(order, "store"); return nil }
	loginDeleteAllTokens = func() ([]string, error) { order = append(order, "wipe"); return nil, nil }
	identityRemove = func() error { order = append(order, "remove-local"); return nil }
	loginLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginLocal, ServerURL: "http://localhost:8000", ActiveWorkspaceID: "ws-local"}, nil
	}
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }

	stdout, err := runLogin(t, "--server-url", "https://mcp.production.flmnt.ai", "--client-id", "c", "--auth-url", "https://a", "--token-url", "https://t")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "wipe,remove-local,store" {
		t.Errorf("the previous session is cleared before the new token is stored; got %v", order)
	}
	if saved.Login != auth.LoginHosted || saved.ServerURL != "https://mcp.production.flmnt.ai" || saved.Sub != "u-123" || saved.Identity != "mike@flmnt.ai" || saved.LoggedInAt != "2026-10-20T16:00:00Z" {
		t.Errorf("session recorded %+v", saved)
	}
	if saved.ActiveWorkspaceID != "" {
		t.Error("a local session's workspace names nothing on the hosted server and must not carry over")
	}
	if !strings.Contains(stdout, "mike@flmnt.ai") || !strings.Contains(stdout, "https://mcp.production.flmnt.ai") {
		t.Errorf("got %q", stdout)
	}
}

/* Re-logging in to the same hosted server keeps the active workspace: it still names something there. */
func TestHostedSessionKeepsTheWorkspaceOnlyForTheSameHostedServer(t *testing.T) {
	prev := auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp", ActiveWorkspaceID: "ws", ActiveWorkspaceName: "sift"}
	same := hostedSession(prev, "https://mcp.production.flmnt.ai", auth.Claims{Sub: "u"}, auth.CLIConfig{}, loginNow())
	if same.ActiveWorkspaceID != "ws" || same.ActiveWorkspaceName != "sift" {
		t.Errorf("same origin keeps the workspace; got %+v", same)
	}
	other := hostedSession(prev, "https://mcp.staging.flmnt.dev", auth.Claims{Sub: "u"}, auth.CLIConfig{}, loginNow())
	if other.ActiveWorkspaceID != "" {
		t.Errorf("another server does not; got %+v", other)
	}
}

func TestLogoutLocalRemovesTheLocalLoginAndItsSession(t *testing.T) {
	stubLogoutSeams(t)
	removed := false
	identityRemove = func() error { removed = true; return nil }
	logoutLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginLocal, ServerURL: defaultLocalURL}, nil
	}
	cleared := false
	logoutClearConfig = func() error { cleared = true; return nil }

	if _, err := runLogout(t, "--local"); err != nil || !removed || !cleared {
		t.Fatalf("err %v removed %v cleared %v", err, removed, cleared)
	}

	cleared = false
	logoutLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginHosted, ServerURL: "https://mcp.production.flmnt.ai"}, nil
	}
	if _, err := runLogout(t, "--local"); err != nil || cleared {
		t.Fatalf("--local leaves a hosted session alone: err %v cleared %v", err, cleared)
	}
}
