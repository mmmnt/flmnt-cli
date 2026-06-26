package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ServerURL string
	ProjectID string // per-repo flmnt project written to the repo config (optional)
	ProxyPort int
	GateCmd   string // UserPromptSubmit: context-recency nudge (default: "flmnt gate")
	BriefCmd  string // SessionStart: inject the project's reasoning state (default: "flmnt brief")
	DeriveCmd string // Stop: derive + import the finished session (default: "flmnt derive --hook")
	Dir       string // project root; defaults to cwd
}

type ProjectConfig struct {
	ServerURL string `json:"server_url"`
	ProjectID string `json:"project_id,omitempty"` // per-repo flmnt project; derive/brief scope to it
}

type mcpServer struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type mcpFile struct {
	McpServers map[string]mcpServer `json:"mcpServers"`
}

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookMatcher struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

// hookScriptsRel is where the bundled .sh nudges/gates land, referenced via $CLAUDE_PROJECT_DIR.
const hookScriptsRel = ".claude/flmnt-hooks"

// kitTools is the flmnt MCP read/write surface the hook map + slash commands drive. setup grants them
// in settings.local.json so the nudged tool calls don't each prompt for permission.
var kitTools = []string{
	"mcp__flmnt__list_streams", "mcp__flmnt__get_stream_metadata", "mcp__flmnt__peek_stream",
	"mcp__flmnt__slice_events", "mcp__flmnt__search_events", "mcp__flmnt__get_graph_neighborhood",
	"mcp__flmnt__read_keyframe", "mcp__flmnt__read_plan", "mcp__flmnt__get_hydration_status",
	"mcp__flmnt__materialize_context", "mcp__flmnt__query_context", "mcp__flmnt__write_keyframe",
	"mcp__flmnt__record_decision", "mcp__flmnt__record_operational_decision",
	"mcp__flmnt__record_exploration", "mcp__flmnt__record_supersession",
	"mcp__flmnt__record_attestation", "mcp__flmnt__record_metric", "mcp__flmnt__record_plan",
	"mcp__flmnt__record_mistake", "mcp__flmnt__create_stream", "mcp__flmnt__hydrate_artifact",
}

func Run(cfg Config) error {
	dir := cfg.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if err := writeProjectConfig(dir, cfg); err != nil {
		return fmt.Errorf("writing .quorum.json: %w", err)
	}
	if err := writeMCPJSON(dir, cfg); err != nil {
		return fmt.Errorf("writing .mcp.json: %w", err)
	}
	if err := writeHookScripts(dir); err != nil {
		return fmt.Errorf("writing hook scripts: %w", err)
	}
	if err := writeCommands(dir); err != nil {
		return fmt.Errorf("writing slash commands: %w", err)
	}
	if err := writeSettingsLocalJSON(dir, cfg); err != nil {
		return fmt.Errorf("writing settings.local.json: %w", err)
	}
	return nil
}

func writeProjectConfig(dir string, cfg Config) error {
	path := filepath.Join(dir, ".quorum.json")
	return writeJSON(path, ProjectConfig{ServerURL: cfg.ServerURL, ProjectID: cfg.ProjectID})
}

func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, ".quorum.json"))
	if err != nil {
		return nil, err
	}
	var pc ProjectConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, err
	}
	return &pc, nil
}

func writeMCPJSON(dir string, cfg Config) error {
	path := filepath.Join(dir, ".mcp.json")

	servers := map[string]mcpServer{
		"quorum": {
			Type: "http",
			URL:  fmt.Sprintf("http://localhost:%d/mcp", cfg.ProxyPort),
		},
	}

	if existing, err := os.ReadFile(path); err == nil {
		var current mcpFile
		if err := json.Unmarshal(existing, &current); err == nil && current.McpServers != nil {
			for k, v := range current.McpServers {
				if k != "quorum" {
					servers[k] = v
				}
			}
		}
	}

	return writeJSON(path, mcpFile{McpServers: servers})
}

// writeCommands materializes the embedded slash-command catalog into .claude/commands/.
func writeCommands(dir string) error {
	return writeEmbeddedDir("assets/commands", filepath.Join(dir, ".claude", "commands"), 0644)
}

// writeHookScripts materializes the embedded nudge/gate scripts into .claude/flmnt-hooks/ (executable).
func writeHookScripts(dir string) error {
	return writeEmbeddedDir("assets/hooks", filepath.Join(dir, hookScriptsRel), 0755)
}

func writeEmbeddedDir(srcDir, dstDir string, mode os.FileMode) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}
	entries, err := assets.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := assets.ReadFile(srcDir + "/" + e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, e.Name()), b, mode); err != nil {
			return err
		}
	}
	return nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// buildHooks is the full automation-kit lifecycle map: cross-platform flmnt CLI commands for the
// read/write/derive/metric/sync moments, plus the bundled .sh nudges/gates referenced via
// $CLAUDE_PROJECT_DIR. `|| true` keeps best-effort CLI hooks from ever failing a session.
func buildHooks(cfg Config) map[string][]hookMatcher {
	brief := orDefault(cfg.BriefCmd, "flmnt brief")
	gate := orDefault(cfg.GateCmd, "flmnt gate")
	derive := orDefault(cfg.DeriveCmd, "flmnt derive --hook")
	script := func(name string) hookEntry {
		return hookEntry{Type: "command", Command: fmt.Sprintf("\"$CLAUDE_PROJECT_DIR/%s/%s\"", hookScriptsRel, name)}
	}
	cmd := func(c string) hookEntry { return hookEntry{Type: "command", Command: c} }
	return map[string][]hookMatcher{
		"SessionStart": {{Matcher: "", Hooks: []hookEntry{cmd(brief), cmd("flmnt health || true")}}},
		"UserPromptSubmit": {{Matcher: "", Hooks: []hookEntry{
			cmd(gate),
			script("mistake-capture-reminder.sh"),
			script("plan-detection.sh"),
			script("supersession-detect.sh"),
			script("mcp-search-reminder.sh"),
		}}},
		"PreToolUse": {{Matcher: "mcp__flmnt__record_decision", Hooks: []hookEntry{script("causal-ref-gate.sh")}}},
		"PostToolUse": {{Matcher: "Bash", Hooks: []hookEntry{cmd("flmnt record-metric --hook")}}},
		"PreCompact": {{Matcher: "", Hooks: []hookEntry{script("precompact-checkpoint.sh"), cmd("flmnt derive --write || true")}}},
		"SubagentStop": {{Matcher: "", Hooks: []hookEntry{cmd("flmnt derive --hook || true")}}},
		"Stop":         {{Matcher: "", Hooks: []hookEntry{cmd(derive)}}},
		"SessionEnd":   {{Matcher: "", Hooks: []hookEntry{cmd("flmnt derive --write || true"), cmd("flmnt sync push || true")}}},
	}
}

// mergePermissions unions the kit's tool allow-list into any existing permissions, preserving the
// user's other allow entries + deny/ask keys.
func mergePermissions(existing any) map[string]any {
	out := map[string]any{}
	seen := map[string]bool{}
	var allow []string
	if m, ok := existing.(map[string]any); ok {
		for k, v := range m {
			out[k] = v
		}
		if raw, ok := m["allow"].([]any); ok {
			for _, a := range raw {
				if s, ok := a.(string); ok && !seen[s] {
					seen[s] = true
					allow = append(allow, s)
				}
			}
		}
	}
	for _, t := range kitTools {
		if !seen[t] {
			seen[t] = true
			allow = append(allow, t)
		}
	}
	out["allow"] = allow
	return out
}

func writeSettingsLocalJSON(dir string, cfg Config) error {
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(claudeDir, "settings.local.json")

	// Start from the existing file (preserve unmanaged keys like enabledMcpjsonServers), then set the
	// managed sections: the full hook map + the merged permission allow-list.
	current := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(existing, &current)
	}
	current["hooks"] = buildHooks(cfg)
	current["permissions"] = mergePermissions(current["permissions"])
	return writeJSON(path, current)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
