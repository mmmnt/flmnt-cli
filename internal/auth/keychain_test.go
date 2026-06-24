package auth

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/99designs/keyring"
)

func fileRing(t *testing.T) keyring.Keyring {
	t.Helper()
	dir := t.TempDir()
	orig := openKeyring
	t.Cleanup(func() { openKeyring = orig })
	ring, err := keyring.Open(keyring.Config{
		ServiceName:      keychainService,
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		FileDir:          dir,
		FilePasswordFunc: filePassword,
	})
	if err != nil {
		t.Fatalf("open ring: %v", err)
	}
	openKeyring = func() (keyring.Keyring, error) { return ring, nil }
	return ring
}

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

func TestTokenKeyNormalizesAcrossServerURLVariants(t *testing.T) {
	fileRing(t)

	// A login keys the token under a workspace-scoped MCP URL...
	const loginURL = "https://mcp.staging.flmnt.dev/mcp?workspace=ws-123"
	if err := StoreToken(loginURL, TokenSet{AccessToken: "broker-at", RefreshToken: "r", IDToken: "i"}); err != nil {
		t.Fatalf("store: %v", err)
	}
	// ...and a read that resolves a different variant (bare host, or /mcp) must find it.
	for _, readURL := range []string{
		"https://mcp.staging.flmnt.dev",
		"https://mcp.staging.flmnt.dev/mcp",
		"https://mcp.staging.flmnt.dev/mcp?workspace=other",
	} {
		got, err := LoadToken(readURL)
		if err != nil {
			t.Fatalf("load %q: %v", readURL, err)
		}
		if got.AccessToken != "broker-at" {
			t.Fatalf("read %q got stale/empty token: %+v", readURL, got)
		}
	}
	// Delete via yet another variant clears the shared entry.
	if err := DeleteToken("https://mcp.staging.flmnt.dev/mcp"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := LoadToken(loginURL); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestNormalizeProjectURLFallsBackOnUnparseableValue(t *testing.T) {
	if got := normalizeProjectURL("not a url"); got != "not a url" {
		t.Fatalf("expected raw fallback, got %q", got)
	}
}

func TestStoreLoadDeleteRoundTrip(t *testing.T) {
	fileRing(t)

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

// Each token must be persisted as its own keyring entry so that no single
// credential blob approaches the Windows Credential Manager 2560-byte cap.
func TestStoreSplitsTokensIntoSeparateEntries(t *testing.T) {
	ring := fileRing(t)
	const url = "https://example.test"

	if err := StoreToken(url, TokenSet{AccessToken: "a", RefreshToken: "r", IDToken: "i"}); err != nil {
		t.Fatalf("store: %v", err)
	}
	for field, want := range map[string]string{
		tokenFieldAccess:  "a",
		tokenFieldRefresh: "r",
		tokenFieldID:      "i",
	} {
		item, err := ring.Get(tokenItemKey(url, field))
		if err != nil {
			t.Fatalf("get %s: %v", field, err)
		}
		if string(item.Data) != want {
			t.Errorf("%s = %q, want %q", field, item.Data, want)
		}
	}
	// The legacy combined-blob key must not be written.
	if _, err := ring.Get(url); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Errorf("legacy combined entry should not exist, got err %v", err)
	}
}

// An empty optional token (id) must not leave a stale entry behind.
func TestStoreClearsEmptyTokenEntry(t *testing.T) {
	ring := fileRing(t)
	const url = "https://example.test"

	if err := StoreToken(url, TokenSet{AccessToken: "a", RefreshToken: "r", IDToken: "i"}); err != nil {
		t.Fatalf("store with id: %v", err)
	}
	if err := StoreToken(url, TokenSet{AccessToken: "a2", RefreshToken: "r2"}); err != nil {
		t.Fatalf("store without id: %v", err)
	}
	if _, err := ring.Get(tokenItemKey(url, tokenFieldID)); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Errorf("stale id entry should be removed, got err %v", err)
	}
	got, err := LoadToken(url)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.AccessToken != "a2" || got.RefreshToken != "r2" || got.IDToken != "" {
		t.Fatalf("unexpected token set: %+v", got)
	}
}

// Tokens written by an older version (single JSON blob under the project key)
// must still load.
func TestLoadFallsBackToLegacyBlob(t *testing.T) {
	ring := fileRing(t)
	const url = "https://example.test"

	blob, _ := json.Marshal(TokenSet{AccessToken: "a", RefreshToken: "r", IDToken: "i"})
	if err := ring.Set(keyring.Item{Key: url, Data: blob}); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	got, err := LoadToken(url)
	if err != nil {
		t.Fatalf("load legacy: %v", err)
	}
	if got.AccessToken != "a" || got.RefreshToken != "r" || got.IDToken != "i" {
		t.Fatalf("legacy round-trip mismatch: %+v", got)
	}
	// A subsequent store migrates off the legacy format.
	if err := StoreToken(url, got); err != nil {
		t.Fatalf("re-store: %v", err)
	}
	if _, err := ring.Get(url); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Errorf("legacy entry should be removed after re-store, got err %v", err)
	}
}
