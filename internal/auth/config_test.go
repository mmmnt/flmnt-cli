package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadConfigRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := SaveConfig(CLIConfig{
		ServerURL:           "https://staging.flmnt.dev",
		ActiveWorkspaceID:   "uuid-1",
		ActiveWorkspaceName: "foo",
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ActiveWorkspaceID != "uuid-1" || cfg.ActiveWorkspaceName != "foo" {
		t.Fatalf("unexpected: %+v", cfg)
	}
}

func TestSaveAndLoadConfigPersistsClientIDAndTokenURL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := SaveConfig(CLIConfig{ClientID: "client-1", TokenURL: "https://auth/oauth2/token"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ClientID != "client-1" || cfg.TokenURL != "https://auth/oauth2/token" {
		t.Fatalf("unexpected: %+v", cfg)
	}
}

func TestLoadConfigReturnsEmptyWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if (cfg != CLIConfig{}) {
		t.Fatalf("expected empty, got %+v", cfg)
	}
}

func TestSaveConfigCreatesDirectoryWith0700Perms(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := SaveConfig(CLIConfig{ActiveWorkspaceID: "uuid-1"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	info, err := os.Stat(filepath.Join(tmp, ".filament"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("expected 0700, got %o", info.Mode().Perm())
	}
}

func TestClearConfigRemovesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	_ = SaveConfig(CLIConfig{ActiveWorkspaceID: "uuid-1"})
	if err := ClearConfig(); err != nil {
		t.Fatalf("ClearConfig: %v", err)
	}
	cfg, _ := LoadConfig()
	if cfg.ActiveWorkspaceID != "" {
		t.Fatalf("expected empty after clear, got %+v", cfg)
	}
}
