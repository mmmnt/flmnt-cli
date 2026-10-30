package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestSetupCommandInstallsSessionStartAndGateHooks(t *testing.T) {
	isolateHookEnv(t)
	c, stdout, _ := hookTestCmd("")
	c.Flags().Int("proxy-port", 9876, "")
	c.Flags().String("corpus-project", "", "")
	c.Flags().Bool("proxy", false, "")
	_ = c.Flags().Set("server-url", "http://localhost:8000/mcp")
	_ = c.Flags().Set("project", "ws-1")
	if err := setupCmd.RunE(c, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(".claude/settings.local.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `session-start"`) || !strings.Contains(string(b), `gate"`) {
		t.Fatalf("settings.local.json lacks the session-start or gate hook: %s", b)
	}
	if !strings.Contains(stdout.String(), "session-start") {
		t.Fatalf("setup's summary must name the session-start hook; got %q", stdout.String())
	}
}
