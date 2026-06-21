package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL string
	ProjectID string // per-repo flmnt project written to the repo config (optional)
	ProxyPort int
	GateCmd   string // UserPromptSubmit: context-recency nudge
	BriefCmd  string // SessionStart: inject the project's reasoning state (read half of the loop)
	DeriveCmd string // Stop: derive + import the finished session (write half of the loop)
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

type settingsFile struct {
	Hooks map[string][]hookMatcher `json:"hooks"`
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

func writeSettingsLocalJSON(dir string, cfg Config) error {
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(claudeDir, "settings.local.json")

	hooks := map[string][]hookMatcher{
		"UserPromptSubmit": {{Matcher: "", Hooks: []hookEntry{{Type: "command", Command: cfg.GateCmd}}}},
	}
	// The continuity loop: SessionStart injects the project's reasoning state; Stop derives the
	// finished session back into flmnt. Both fail quiet, so they never disrupt a session.
	if cfg.BriefCmd != "" {
		hooks["SessionStart"] = []hookMatcher{{Matcher: "", Hooks: []hookEntry{{Type: "command", Command: cfg.BriefCmd}}}}
	}
	if cfg.DeriveCmd != "" {
		hooks["Stop"] = []hookMatcher{{Matcher: "", Hooks: []hookEntry{{Type: "command", Command: cfg.DeriveCmd}}}}
	}
	settings := settingsFile{Hooks: hooks}

	// Preserve existing keys if file already exists
	if existing, err := os.ReadFile(path); err == nil {
		var current map[string]json.RawMessage
		if err := json.Unmarshal(existing, &current); err == nil {
			current["hooks"] = mustMarshal(settings.Hooks)
			return writeJSON(path, current)
		}
	}

	return writeJSON(path, settings)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
