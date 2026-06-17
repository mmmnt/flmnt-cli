package cmd

import (
	"strings"
	"testing"
)

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
	if ep.MCPURL != defaultLocalURL || ep.Workspace != defaultLocalWorkspace {
		t.Fatalf("defaults wrong: %s / %s", ep.MCPURL, ep.Workspace)
	}
	if !strings.Contains(ep.AuthValue, "Bearer L") {
		t.Fatalf("auth value = %q", ep.AuthValue)
	}
}
