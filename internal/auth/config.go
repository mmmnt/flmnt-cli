package auth

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type CLIConfig struct {
	ServerURL           string `json:"server_url,omitempty"`
	ActiveWorkspaceID   string `json:"active_workspace_id,omitempty"`
	ActiveWorkspaceName string `json:"active_workspace_name,omitempty"`
	ClientID            string `json:"client_id,omitempty"`
	TokenURL            string `json:"token_url,omitempty"`
	AuthURL             string `json:"auth_url,omitempty"`
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".filament"), nil
}

func configPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig() (CLIConfig, error) {
	path, err := configPath()
	if err != nil {
		return CLIConfig{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CLIConfig{}, nil
		}
		return CLIConfig{}, err
	}
	var cfg CLIConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return CLIConfig{}, err
	}
	return cfg, nil
}

func SaveConfig(cfg CLIConfig) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func ClearConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
