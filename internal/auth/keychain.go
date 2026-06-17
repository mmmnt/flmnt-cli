package auth

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/99designs/keyring"
)

const keychainService = "quorum-cli"

var ErrNotFound = errors.New("no token stored for this project")

type TokenSet struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
}

func StoreToken(projectURL string, tokens TokenSet) error {
	ring, err := openKeyring()
	if err != nil {
		return err
	}
	data, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	return ring.Set(keyring.Item{
		Key:  projectURL,
		Data: data,
	})
}

func LoadToken(projectURL string) (TokenSet, error) {
	ring, err := openKeyring()
	if err != nil {
		return TokenSet{}, err
	}
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
	ring, err := openKeyring()
	if err != nil {
		return err
	}
	if err := ring.Remove(projectURL); err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
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
