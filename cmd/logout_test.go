package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/pflag"
)

func stubLogoutSeams(t *testing.T) {
	t.Helper()
	origLoad, origClear, origTok, origWipe, origRevoke, origRemove := logoutLoadConfig, logoutClearConfig, logoutLoadToken, logoutDeleteAllTokens, logoutRevoke, identityRemove
	logoutLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	logoutClearConfig = func() error { return nil }
	logoutLoadToken = func(string) (auth.TokenSet, error) { return auth.TokenSet{}, auth.ErrNotFound }
	logoutDeleteAllTokens = func() ([]string, error) { return nil, nil }
	logoutRevoke = func(string, string, string) error { return nil }
	identityRemove = func() error { return nil }
	t.Cleanup(func() {
		logoutLoadConfig, logoutClearConfig, logoutLoadToken, logoutDeleteAllTokens, logoutRevoke, identityRemove = origLoad, origClear, origTok, origWipe, origRevoke, origRemove
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
Logout clears everything. The config had been cleared by an earlier logout, so the next one resolved
no server URL, deleted no token, and still said "Logged out." — whoami kept answering with the email
from the production token left in the keychain. Now every token for every server goes, the local
login goes, the config goes, and the line says so.
*/
func TestLogoutClearsEveryTokenTheLocalLoginAndTheSessionEvenWithNoConfig(t *testing.T) {
	stubLogoutSeams(t)
	wiped, removed, cleared := false, false, false
	logoutDeleteAllTokens = func() ([]string, error) {
		wiped = true
		return []string{"https://mcp.production.flmnt.ai|access_token", "https://mcp.production.flmnt.ai|refresh_token", "https://mcp.staging.flmnt.dev|access_token"}, nil
	}
	identityRemove = func() error { removed = true; return nil }
	logoutClearConfig = func() error { cleared = true; return nil }

	stdout, err := runLogout(t)
	if err != nil {
		t.Fatal(err)
	}
	if !wiped || !removed || !cleared {
		t.Fatalf("wiped %v removed %v cleared %v", wiped, removed, cleared)
	}
	for _, want := range []string{"Logged out.", "mcp.production.flmnt.ai", "mcp.staging.flmnt.dev", "Nothing remains"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("want %q in %q", want, stdout)
		}
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
