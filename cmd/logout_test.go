package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/spf13/pflag"
)

func stubLogoutSeams(t *testing.T) {
	t.Helper()
	origLoad, origClear, origTok, origWipe, origRevoke := logoutLoadConfig, logoutClearConfig, logoutLoadToken, logoutDeleteAllTokens, logoutRevoke
	logoutLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	logoutClearConfig = func() error { return nil }
	logoutLoadToken = func(string) (auth.TokenSet, error) { return auth.TokenSet{}, auth.ErrNotFound }
	logoutDeleteAllTokens = func() ([]string, error) { return nil, nil }
	logoutRevoke = func(string, string, string) error { return nil }
	t.Cleanup(func() {
		logoutLoadConfig, logoutClearConfig, logoutLoadToken, logoutDeleteAllTokens, logoutRevoke = origLoad, origClear, origTok, origWipe, origRevoke
	})
}

func runLogout(t *testing.T, args ...string) (string, error) {
	t.Helper()
	logoutCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	t.Setenv("QUORUM_SERVER_URL", "")
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(append([]string{"logout"}, args...))
	err := rootCmd.Execute()
	return out.String(), err
}

/*
Logout clears the session and every token. The config had been cleared by an earlier logout, so the
next one resolved no server URL, deleted no token, and still said "Logged out." — whoami kept
answering with the email from the production token left in the keychain. Now every token for every
server goes, the config goes, and the line says so. The local IDENTITY stays: it is who this machine
is, not a login, and the line says that too (founder ruling 2026-10-20: identity and session are
separate).
*/
func TestLogoutClearsEveryTokenAndTheSessionAndKeepsTheLocalIdentity(t *testing.T) {
	stubLogoutSeams(t)
	t.Setenv("HOME", t.TempDir())
	if err := identity.Save(localMike); err != nil {
		t.Fatal(err)
	}
	wiped, cleared := false, false
	logoutDeleteAllTokens = func() ([]string, error) {
		wiped = true
		return []string{"https://mcp.production.flmnt.ai|access_token", "https://mcp.production.flmnt.ai|refresh_token", "https://mcp.staging.flmnt.dev|access_token"}, nil
	}
	logoutClearConfig = func() error { cleared = true; return nil }

	stdout, err := runLogout(t)
	if err != nil {
		t.Fatal(err)
	}
	if !wiped || !cleared {
		t.Fatalf("wiped %v cleared %v", wiped, cleared)
	}
	if kept, err := identity.Load(); err != nil || kept.Sub != localMike.Sub {
		t.Errorf("the local identity must survive a logout; got %+v %v", kept, err)
	}
	for _, want := range []string{"Logged out.", "mcp.production.flmnt.ai", "mcp.staging.flmnt.dev", "The local identity " + localMike.Sub + " stays"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("want %q in %q", want, stdout)
		}
	}
	if strings.Contains(stdout, "Nothing remains") {
		t.Errorf("the line must not claim nothing remains while the identity does: %q", stdout)
	}
}

/* With no local identity on the machine, the line says nothing remains. */
func TestLogoutLineSaysNothingRemainsOnlyWithoutAnIdentity(t *testing.T) {
	stubLogoutSeams(t)
	t.Setenv("HOME", t.TempDir())
	stdout, err := runLogout(t)
	if err != nil || !strings.Contains(stdout, "Nothing remains.") {
		t.Fatalf("err %v stdout %q", err, stdout)
	}
}

/* The hosted refresh token is revoked server-side first, from what the login recorded, before it is deleted. */
func TestLogoutRevokesTheRecordedHostedLoginBeforeClearing(t *testing.T) {
	stubLogoutSeams(t)
	logoutLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginHosted, ServerURL: "https://mcp.production.flmnt.ai", Identity: "mike@flmnt.ai", ClientID: "c", RevocationEndpoint: "https://broker/revoke"}, nil
	}
	logoutLoadToken = func(u string) (auth.TokenSet, error) {
		if u != "https://mcp.production.flmnt.ai" {
			t.Errorf("token for %q", u)
		}
		return auth.TokenSet{RefreshToken: "ref"}, nil
	}
	order := []string{}
	logoutRevoke = func(url, client, token string) error {
		if url != "https://broker/revoke" || client != "c" || token != "ref" {
			t.Errorf("revoke(%s,%s,%s)", url, client, token)
		}
		order = append(order, "revoke")
		return nil
	}
	logoutDeleteAllTokens = func() ([]string, error) { order = append(order, "wipe"); return nil, nil }

	stdout, err := runLogout(t)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "revoke,wipe" {
		t.Errorf("got %v", order)
	}
	if !strings.Contains(stdout, "mike@flmnt.ai") {
		t.Errorf("the line names the login it ended; got %q", stdout)
	}
}
