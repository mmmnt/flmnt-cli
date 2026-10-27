package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mmmnt/flmnt-cli/internal/home"
	"io/fs"
	"os"
	"path/filepath"
)

// The CLI holds ONE login at a time. Login names which kind the recorded session is — LoginHosted
// (`flmnt login`, OAuth tokens in the keychain) or LoginLocal (`flmnt login --local`, the derived
// identity) — and a login of either kind replaces whatever session was there. A config written by an
// older build carries no Login; it was only ever written by a hosted login.
const (
	LoginHosted = "hosted"
	LoginLocal  = "local"
)

type CLIConfig struct {
	// Login is LoginHosted or LoginLocal: the kind of session this config records.
	Login string `json:"login,omitempty"`
	// ServerURL is the server the session was made against: the hosted MCP for a hosted login, the
	// loopback server for a local one. Every command resolves its upstream from here.
	ServerURL string `json:"server_url,omitempty"`
	// Sub and Identity are who the session is: the token's sub and its display name (email, username
	// or sub) for a hosted login; the derived sub, twice, for a local one.
	Sub        string `json:"sub,omitempty"`
	Identity   string `json:"identity,omitempty"`
	LoggedInAt string `json:"logged_in_at,omitempty"`

	ActiveWorkspaceID   string `json:"active_workspace_id,omitempty"`
	ActiveWorkspaceName string `json:"active_workspace_name,omitempty"`
	ClientID            string `json:"client_id,omitempty"`
	TokenURL            string `json:"token_url,omitempty"`
	AuthURL             string `json:"auth_url,omitempty"`
	RevocationEndpoint  string `json:"revocation_endpoint,omitempty"`
}

func ConfigDir() (string, error) {
	return home.Dir()
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
	if existing, err := os.ReadFile(path); err == nil && !json.Valid(existing) {
		return fmt.Errorf("%s is not valid JSON; fix or remove it and try again (it was left as it was)", path)
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Session says whether a login is recorded at all. An older config with a server_url but no login
// field was written by a hosted login and counts as one.
func (c CLIConfig) Session() bool {
	return c.Login != "" || c.ServerURL != ""
}

// LoginKind is the session's kind, inferring LoginHosted for a config an older build wrote.
func (c CLIConfig) LoginKind() string {
	if c.Login == "" && c.ServerURL != "" {
		return LoginHosted
	}
	return c.Login
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
