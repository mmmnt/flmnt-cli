package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func TestResolveAuthCmd(t *testing.T) {
	// An explicit command is honored verbatim, no project config required.
	if got, err := resolveAuthCmd("my-cmd", t.TempDir()); err != nil || got != "my-cmd" {
		t.Fatalf("explicit flag: got %q err %v", got, err)
	}
	// The default runs inside a configured flmnt project (.quorum.json present).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".quorum.json"), []byte(`{"serverUrl":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveAuthCmd("", dir); err != nil || got != defaultLocalAuthCmd {
		t.Fatalf("configured project: got %q err %v", got, err)
	}
	// The default is REFUSED in an unconfigured directory (a foreign repo), so its cwd script can't run.
	if _, err := resolveAuthCmd("", t.TempDir()); err == nil {
		t.Fatal("expected refusal to run the default cwd script without a .quorum.json")
	}
}

func TestRunAuthHelperExtractsAuthorizationHeader(t *testing.T) {
	got, err := runAuthHelper(`echo '{"Authorization": "Bearer LOCALTOK", "X-Workspace-Id": "quorum"}'`)
	if err != nil {
		t.Fatalf("runAuthHelper: %v", err)
	}
	if got != "Bearer LOCALTOK" {
		t.Fatalf("got %q, want Bearer LOCALTOK", got)
	}
}

func TestRunAuthHelperErrorsWhenNoAuthorizationHeader(t *testing.T) {
	if _, err := runAuthHelper(`echo '{"X-Workspace-Id": "quorum"}'`); err == nil {
		t.Fatal("expected error when no Authorization header present")
	}
}

func TestRunAuthHelperErrorsOnNonJSON(t *testing.T) {
	if _, err := runAuthHelper(`echo not-json`); err == nil {
		t.Fatal("expected error on non-JSON helper output")
	}
}

func TestSyncCommandsAreRegisteredWithExpectedFlags(t *testing.T) {
	sub := map[string]bool{}
	for _, c := range syncCmd.Commands() {
		sub[c.Name()] = true
		if c.Flags().Lookup("dry-run") == nil {
			t.Fatalf("%s missing --dry-run flag", c.Name())
		}
		if c.Flags().Lookup("local-auth-cmd") == nil {
			t.Fatalf("%s missing --local-auth-cmd flag", c.Name())
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

func TestResolveLocalEndpointUsesAuthCmdAndDefaults(t *testing.T) {
	cmd := syncPushCmd
	if err := cmd.Flags().Set("local-auth-cmd", `echo '{"Authorization": "Bearer L"}'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Flags().Set("local-auth-cmd", defaultLocalAuthCmd) })

	ep, err := resolveLocalEndpoint(cmd)
	if err != nil {
		t.Fatalf("resolveLocalEndpoint: %v", err)
	}
	if ep.Ref != defaultLocalURL || ep.Workspace != defaultLocalWorkspace {
		t.Fatalf("defaults wrong: %s / %s", ep.Ref, ep.Workspace)
	}
	if ep.GQL == nil {
		t.Fatal("expected an authenticated GraphQL client from the local auth helper")
	}
}
