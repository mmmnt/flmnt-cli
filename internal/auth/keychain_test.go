package auth

import (
	"errors"
	"testing"

	"github.com/99designs/keyring"
)

func TestKeychainConfigPrefersOSBackendsWithFileFallbackLast(t *testing.T) {
	cfg := keychainConfig()
	if len(cfg.AllowedBackends) < 2 {
		t.Fatal("AllowedBackends must list the OS keychains plus a file fallback")
	}
	last := cfg.AllowedBackends[len(cfg.AllowedBackends)-1]
	if last != keyring.FileBackend {
		t.Errorf("file backend must be the LAST-resort fallback, got %q", last)
	}
	osOnly := map[keyring.BackendType]bool{
		keyring.KeychainBackend:      true,
		keyring.WinCredBackend:       true,
		keyring.SecretServiceBackend: true,
	}
	for _, b := range cfg.AllowedBackends[:len(cfg.AllowedBackends)-1] {
		if !osOnly[b] {
			t.Errorf("only OS keychain backends may precede the file fallback, got %q", b)
		}
	}
	if cfg.FileDir == "" {
		t.Error("FileDir must be set for the file fallback")
	}
}

func TestStoreLoadDeleteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	orig := openKeyring
	t.Cleanup(func() { openKeyring = orig })
	openKeyring = func() (keyring.Keyring, error) {
		return keyring.Open(keyring.Config{
			ServiceName:      keychainService,
			AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
			FileDir:          dir,
			FilePasswordFunc: filePassword,
		})
	}

	const url = "https://example.test"
	if _, err := LoadToken(url); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound before store, got %v", err)
	}
	if err := StoreToken(url, TokenSet{AccessToken: "a", RefreshToken: "r", IDToken: "i"}); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := LoadToken(url)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.AccessToken != "a" || got.RefreshToken != "r" || got.IDToken != "i" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if err := DeleteToken(url); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := LoadToken(url); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := DeleteToken(url); err != nil {
		t.Fatalf("delete must be idempotent, got %v", err)
	}
}
