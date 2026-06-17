package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/setup"
)

func TestSetupWritesMCPJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := setup.Config{
		ServerURL: "https://quorum.example.com",
		ProxyPort: 9876,
		GateCmd:   "quorum gate",
		Dir:       dir,
	}

	if err := setup.Run(cfg); err != nil {
		t.Fatalf("setup.Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("expected .mcp.json to exist: %v", err)
	}
	content := string(data)
	if !contains(content, "localhost:9876") {
		t.Error("expected proxy URL in .mcp.json")
	}
}

func TestSetupWritesHookToSettingsLocalJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := setup.Config{
		ServerURL: "https://quorum.example.com",
		ProxyPort: 9876,
		GateCmd:   "quorum gate",
		Dir:       dir,
	}

	if err := setup.Run(cfg); err != nil {
		t.Fatalf("setup.Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("expected settings.local.json to exist: %v", err)
	}
	if !contains(string(data), "quorum gate") {
		t.Error("expected gate command in settings.local.json")
	}
}

func TestSetupWritesQuorumJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := setup.Config{
		ServerURL: "https://quorum.example.com",
		ProxyPort: 9876,
		GateCmd:   "quorum gate",
		Dir:       dir,
	}

	if err := setup.Run(cfg); err != nil {
		t.Fatalf("setup.Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".quorum.json"))
	if err != nil {
		t.Fatalf("expected .quorum.json to exist: %v", err)
	}
	if !contains(string(data), "https://quorum.example.com") {
		t.Error("expected server URL in .quorum.json")
	}
}

func TestSetupMergesMCPJSONPreservingExistingServers(t *testing.T) {
	dir := t.TempDir()
	existing := `{"mcpServers":{"other":{"type":"http","url":"http://other:8080"}}}`
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(existing), 0644); err != nil {
		t.Fatalf("writing existing .mcp.json: %v", err)
	}

	cfg := setup.Config{
		ServerURL: "https://quorum.example.com",
		ProxyPort: 9876,
		GateCmd:   "quorum gate",
		Dir:       dir,
	}
	if err := setup.Run(cfg); err != nil {
		t.Fatalf("setup.Run: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	content := string(data)
	if !contains(content, "other") {
		t.Error("expected existing 'other' server to be preserved in .mcp.json")
	}
	if !contains(content, "localhost:9876") {
		t.Error("expected quorum proxy URL to be present in .mcp.json")
	}
}

func TestSetupIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := setup.Config{
		ServerURL: "https://quorum.example.com",
		ProxyPort: 9876,
		GateCmd:   "quorum gate",
		Dir:       dir,
	}

	if err := setup.Run(cfg); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := setup.Run(cfg); err != nil {
		t.Fatalf("second run (idempotent): %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
