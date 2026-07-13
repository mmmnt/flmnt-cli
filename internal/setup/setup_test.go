package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func readMCPServers(t *testing.T, dir string) map[string]map[string]any {
	t.Helper()
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
	return f.McpServers
}

const richMCPSeed = `{
  "mcpServers": {
    "atlassian": { "type": "stdio", "command": "uvx", "args": ["mcp-atlassian"], "env": { "JIRA_URL": "https://x/" } },
    "quorum": { "type": "http", "url": "http://localhost:8000/mcp", "headersHelper": "bash scripts/h.sh", "headers": { "X-Workspace-Id": "quorum" } }
  }
}`

// Default (direct) mode: write a direct `flmnt` entry to ServerURL, no proxy, preserve others.
func TestRunDirectModePreservesServersAndWritesDirectEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(richMCPSeed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(Config{ServerURL: "https://mcp.staging.flmnt.dev/mcp?workspace=w", ProjectID: "p", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s := readMCPServers(t, dir)

	// stdio command/args/env preserved verbatim — the old {type,url} round-trip dropped these.
	if at := s["atlassian"]; at["command"] != "uvx" || at["args"] == nil || at["env"] == nil {
		t.Fatalf("atlassian not preserved: %v", at)
	}
	// existing `quorum` entry untouched (url + headersHelper intact).
	if q := s["quorum"]; q["url"] != "http://localhost:8000/mcp" || q["headersHelper"] == nil {
		t.Fatalf("quorum clobbered: %v", q)
	}
	// managed direct entry written to ServerURL.
	if f := s["flmnt"]; f == nil || f["url"] != "https://mcp.staging.flmnt.dev/mcp?workspace=w" {
		t.Fatalf("direct flmnt entry wrong: %v", f)
	}
	// no proxy entry in default mode.
	if _, ok := s["flmnt-proxy"]; ok {
		t.Fatalf("flmnt-proxy should not be written in direct mode: %v", s["flmnt-proxy"])
	}
}

// --proxy mode: write the `flmnt-proxy` localhost entry, preserve others.
func TestRunProxyModeWritesProxyEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(richMCPSeed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(Config{ServerURL: "u", ProjectID: "p", ProxyPort: 9876, Proxy: true, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s := readMCPServers(t, dir)

	if at := s["atlassian"]; at["command"] != "uvx" {
		t.Fatalf("atlassian not preserved: %v", at)
	}
	if p := s["flmnt-proxy"]; p == nil || p["url"] != "http://localhost:9876/mcp" {
		t.Fatalf("flmnt-proxy entry wrong: %v", p)
	}
}

// Switching from --proxy back to direct removes the stale setup-managed flmnt-proxy entry.
func TestRunDirectModeDropsStaleProxyEntry(t *testing.T) {
	dir := t.TempDir()
	if err := Run(Config{ServerURL: "https://r/mcp", ProxyPort: 9876, Proxy: true, Dir: dir}); err != nil {
		t.Fatalf("Run proxy: %v", err)
	}
	if err := Run(Config{ServerURL: "https://r/mcp", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run direct: %v", err)
	}
	s := readMCPServers(t, dir)
	if _, ok := s["flmnt-proxy"]; ok {
		t.Fatalf("stale flmnt-proxy not removed when switching to direct: %v", s)
	}
	if f := s["flmnt"]; f == nil || f["url"] != "https://r/mcp" {
		t.Fatalf("direct flmnt entry wrong: %v", f)
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

func TestPrecompactHookNeverFabricatesStreamIDs(t *testing.T) {
	b, err := assets.ReadFile("assets/hooks/precompact-checkpoint.sh")
	if err != nil {
		t.Fatalf("read embedded hook: %v", err)
	}
	script := string(b)
	if strings.Contains(script, `${_root//\//-}`) {
		t.Fatal("precompact hook derives a stream id from the filesystem path; stream ids are strict and must be resolved via list_streams")
	}
	if !strings.Contains(script, "list_streams") {
		t.Fatal("precompact hook must direct the model to resolve stream ids via list_streams")
	}
}
