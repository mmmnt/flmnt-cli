package auth

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"path/filepath"

	"github.com/99designs/keyring"
)

const keychainService = "quorum-cli"

// Token fields are stored as separate keyring entries. Windows Credential
// Manager caps a single credential blob at 2560 bytes
// (CRED_MAX_CREDENTIAL_BLOB_SIZE); the combined Cognito JWT set (access +
// refresh + id) routinely exceeds that, so writing them as one JSON blob fails
// with "the stub received bad data" (RPC_X_BAD_STUB_DATA). Each token on its
// own stays well under the limit.
const (
	tokenFieldAccess  = "access_token"
	tokenFieldRefresh = "refresh_token"
	tokenFieldID      = "id_token"
)

var ErrNotFound = errors.New("no token stored for this project")

type TokenSet struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
}

// normalizeProjectURL keys tokens by server origin (scheme://host) so the auth
// identity is shared across server-URL variants for the same host — a `/mcp`
// path or a `?workspace=<id>` query must not split the credential into separate
// entries (which silently strands a fresh login under one key while reads hit a
// stale token under another). Falls back to the raw value if it can't be parsed.
func normalizeProjectURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}
	return u.Scheme + "://" + u.Host
}

func tokenItemKey(projectURL, field string) string {
	return projectURL + "|" + field
}

func StoreToken(projectURL string, tokens TokenSet) error {
	projectURL = normalizeProjectURL(projectURL)
	ring, err := openKeyring()
	if err != nil {
		return err
	}
	fields := []struct {
		key string
		val string
	}{
		{tokenItemKey(projectURL, tokenFieldAccess), tokens.AccessToken},
		{tokenItemKey(projectURL, tokenFieldRefresh), tokens.RefreshToken},
		{tokenItemKey(projectURL, tokenFieldID), tokens.IDToken},
	}
	for _, f := range fields {
		if f.val == "" {
			// Drop any stale value so partial sets don't linger.
			_ = ring.Remove(f.key)
			continue
		}
		if err := ring.Set(keyring.Item{Key: f.key, Data: []byte(f.val)}); err != nil {
			return err
		}
	}
	// Remove any legacy single-blob entry written by older versions.
	_ = ring.Remove(projectURL)
	return nil
}

func LoadToken(projectURL string) (TokenSet, error) {
	projectURL = normalizeProjectURL(projectURL)
	ring, err := openKeyring()
	if err != nil {
		return TokenSet{}, err
	}
	access, err := getItem(ring, tokenItemKey(projectURL, tokenFieldAccess))
	if errors.Is(err, keyring.ErrKeyNotFound) {
		// Fall back to the legacy combined-blob format.
		return loadLegacyToken(ring, projectURL)
	}
	if err != nil {
		return TokenSet{}, err
	}
	refresh, err := getItem(ring, tokenItemKey(projectURL, tokenFieldRefresh))
	if err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return TokenSet{}, err
	}
	id, err := getItem(ring, tokenItemKey(projectURL, tokenFieldID))
	if err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return TokenSet{}, err
	}
	return TokenSet{AccessToken: access, RefreshToken: refresh, IDToken: id}, nil
}

func getItem(ring keyring.Keyring, key string) (string, error) {
	item, err := ring.Get(key)
	if err != nil {
		return "", err
	}
	return string(item.Data), nil
}

func loadLegacyToken(ring keyring.Keyring, projectURL string) (TokenSet, error) {
	item, err := ring.Get(projectURL)
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return TokenSet{}, ErrNotFound
		}
		return TokenSet{}, err
	}
	var t TokenSet
	if err := json.Unmarshal(item.Data, &t); err != nil {
		return TokenSet{}, err
	}
	return t, nil
}

func DeleteToken(projectURL string) error {
	projectURL = normalizeProjectURL(projectURL)
	ring, err := openKeyring()
	if err != nil {
		return err
	}
	keys := []string{
		tokenItemKey(projectURL, tokenFieldAccess),
		tokenItemKey(projectURL, tokenFieldRefresh),
		tokenItemKey(projectURL, tokenFieldID),
		projectURL, // legacy single-blob entry
	}
	for _, key := range keys {
		if err := ring.Remove(key); err != nil {
			if errors.Is(err, keyring.ErrKeyNotFound) || errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func filePassword(string) (string, error) { return keychainService, nil }

func keychainConfig() keyring.Config {
	fileDir := ""
	if dir, err := ConfigDir(); err == nil {
		fileDir = filepath.Join(dir, "keyring")
	}
	return keyring.Config{
		ServiceName: keychainService,
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,      // macOS Keychain (requires CGO)
			keyring.WinCredBackend,       // Windows Credential Manager
			keyring.SecretServiceBackend, // Linux Secret Service (D-Bus)
			keyring.FileBackend,          // last-resort fallback (CGO-free builds, headless, CI)
		},
		FileDir:          fileDir,
		FilePasswordFunc: filePassword,
	}
}

var openKeyring = func() (keyring.Keyring, error) {
	return keyring.Open(keychainConfig())
}
