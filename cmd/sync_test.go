package cmd

import (
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func TestSyncCommandsAreRegisteredWithExpectedFlags(t *testing.T) {
	sub := map[string]bool{}
	for _, c := range syncCmd.Commands() {
		sub[c.Name()] = true
		if c.Flags().Lookup("dry-run") == nil {
			t.Fatalf("%s missing --dry-run flag", c.Name())
		}
		if c.Flags().Lookup("local-auth-cmd") != nil {
			t.Fatalf("%s still carries --local-auth-cmd; the helper it named is retired", c.Name())
		}
	}
	if !sub["push"] || !sub["pull"] {
		t.Fatalf("expected push and pull subcommands, got %v", sub)
	}
}

func TestResolveRemoteServerURLFallsBackToLoginConfig(t *testing.T) {
	t.Setenv("QUORUM_SERVER_URL", "")
	orig := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.staging.flmnt.dev"}, nil
	}
	t.Cleanup(func() { authHeaderLoadConfig = orig })

	if got := resolveRemoteServerURL(syncPushCmd); got != "https://mcp.staging.flmnt.dev" {
		t.Fatalf("resolveRemoteServerURL = %q, want the login-config ServerURL", got)
	}
}

func TestResolveRemoteServerURLPrefersRemoteURLFlag(t *testing.T) {
	if err := syncPushCmd.Flags().Set("remote-url", "https://override.example"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syncPushCmd.Flags().Set("remote-url", "") })

	if got := resolveRemoteServerURL(syncPushCmd); got != "https://override.example" {
		t.Fatalf("resolveRemoteServerURL = %q, want the --remote-url override", got)
	}
}

/*
sync's local side ran the checkout's headers-helper (quorum's scripts/mcp-auth-header.sh) for its

	bearer and took the workspace from the X-Workspace-Id it printed. The script is gone: the local
	bearer is the keychain token `flmnt login --local` stores under the local server URL, and the
	workspace is --local-workspace, default "quorum".
*/
func TestResolveLocalEndpointAuthenticatesFromTheKeychain(t *testing.T) {
	stubKeychain(t, freshJWT(t, "local-login"), nil)

	ep, err := resolveLocalEndpoint(syncPushCmd)
	if err != nil {
		t.Fatalf("resolveLocalEndpoint: %v", err)
	}
	if ep.Ref != defaultLocalURL || ep.Workspace != defaultLocalWorkspace {
		t.Fatalf("defaults wrong: %s / %s", ep.Ref, ep.Workspace)
	}
	if ep.GQL == nil {
		t.Fatal("expected an authenticated GraphQL client from the stored local token")
	}
}

/* No stored token for the local server: refused with the login that fills it, nothing substituted. */
func TestResolveLocalEndpointRefusesWithoutALocalLogin(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)

	_, err := resolveLocalEndpoint(syncPushCmd)
	want := "not logged in to http://localhost:8000: run `flmnt login --local --server-url http://localhost:8000`"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}
