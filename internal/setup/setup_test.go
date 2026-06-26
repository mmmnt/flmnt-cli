package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	return s
}

func TestRunInstallsCommandsHooksAndFullMap(t *testing.T) {
	dir := t.TempDir()
	if err := Run(Config{ServerURL: "https://x/mcp", ProjectID: "p", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 13 slash commands written.
	cmds, err := os.ReadDir(filepath.Join(dir, ".claude", "commands"))
	if err != nil {
		t.Fatalf("read commands: %v", err)
	}
	if len(cmds) != 13 {
		t.Fatalf("commands = %d, want 13", len(cmds))
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "commands", "flmnt-recall.md")); err != nil {
		t.Fatalf("flmnt-recall.md missing: %v", err)
	}

	// Hook scripts written + executable.
	gate := filepath.Join(dir, hookScriptsRel, "causal-ref-gate.sh")
	fi, err := os.Stat(gate)
	if err != nil {
		t.Fatalf("causal-ref-gate.sh missing: %v", err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Fatalf("hook script not executable: %v", fi.Mode())
	}

	// Full lifecycle hook map.
	s := readSettings(t, dir)
	hooks, ok := s["hooks"].(map[string]any)
	if !ok {
		t.Fatal("no hooks map")
	}
	for _, ev := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PreCompact", "SubagentStop", "Stop", "SessionEnd"} {
		if _, ok := hooks[ev]; !ok {
			t.Fatalf("missing hook event %s", ev)
		}
	}

	// Permissions grant the kit tools.
	perms := s["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	if len(allow) < len(kitTools) {
		t.Fatalf("allow = %d, want >= %d", len(allow), len(kitTools))
	}
}

func TestRunPreservesRicherMCPServersAndWritesProxy(t *testing.T) {
	dir := t.TempDir()
	seed := `{
  "mcpServers": {
    "atlassian": { "type": "stdio", "command": "uvx", "args": ["mcp-atlassian"], "env": { "JIRA_URL": "https://x/" } },
    "quorum": { "type": "http", "url": "http://localhost:8000/mcp", "headersHelper": "bash scripts/h.sh", "headers": { "X-Workspace-Id": "quorum" } },
    "flmnt": { "type": "http", "url": "https://mcp.staging.flmnt.dev/mcp?workspace=w" }
  }
}`
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(Config{ServerURL: "u", ProjectID: "p", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		McpServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// atlassian (stdio) preserved verbatim — the old {type,url} round-trip dropped these.
	at := f.McpServers["atlassian"]
	if at["command"] != "uvx" || at["args"] == nil || at["env"] == nil {
		t.Fatalf("atlassian command/args/env not preserved: %v", at)
	}
	// existing `quorum` entry left untouched — not clobbered by the managed proxy.
	q := f.McpServers["quorum"]
	if q["url"] != "http://localhost:8000/mcp" || q["headersHelper"] == nil {
		t.Fatalf("existing quorum entry clobbered: %v", q)
	}
	// direct `flmnt` entry preserved.
	if f.McpServers["flmnt"]["url"] != "https://mcp.staging.flmnt.dev/mcp?workspace=w" {
		t.Fatalf("direct flmnt entry changed: %v", f.McpServers["flmnt"])
	}
	// the managed proxy entry is named `flmnt-proxy` at the configured port.
	p := f.McpServers["flmnt-proxy"]
	if p == nil || p["url"] != "http://localhost:9876/mcp" {
		t.Fatalf("flmnt-proxy entry wrong: %v", p)
	}
}

func TestRunPreservesExistingSettingsAndMergesPermissions(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"enabledMcpjsonServers":["atlassian"],"permissions":{"allow":["custom__tool"],"deny":["bad__tool"]}}`
	if err := os.WriteFile(filepath.Join(claude, "settings.local.json"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(Config{ServerURL: "u", ProjectID: "p", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	s := readSettings(t, dir)
	if _, ok := s["enabledMcpjsonServers"]; !ok {
		t.Fatal("dropped unmanaged key enabledMcpjsonServers")
	}
	perms := s["permissions"].(map[string]any)
	if perms["deny"] == nil {
		t.Fatal("dropped existing permissions.deny")
	}
	allow := perms["allow"].([]any)
	has := func(x string) bool {
		for _, a := range allow {
			if a == x {
				return true
			}
		}
		return false
	}
	if !has("custom__tool") {
		t.Fatal("dropped existing allow entry")
	}
	if !has("mcp__flmnt__record_metric") {
		t.Fatal("did not merge kit tool into allow")
	}
}
