package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/99designs/keyring"
)

/*
The identity's keyring is not the tokens' keyring (QRM-ADR-016 §5f, founder ruling 2026-10-20): a

	logout removes every token under the CLI's service and the file keyring directory whole, and the
	machine identity must survive it. Both rings on the file backend, under one HOME: the sweep empties
	the tokens' ring and directory and leaves the identity's item and directory untouched.
*/
func TestDeleteAllTokensLeavesTheIdentityRingAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	orig := goosForKeychain
	goosForKeychain = "linux"
	t.Cleanup(func() { goosForKeychain = orig })
	tokens := fileRing(t)
	if err := tokens.Set(keyring.Item{Key: "https://mcp.production.flmnt.ai|access_token", Data: []byte("t")}); err != nil {
		t.Fatal(err)
	}
	identityDir := filepath.Join(home, ".filament", "identity")
	ring, err := keyring.Open(keyring.Config{ServiceName: identityService, AllowedBackends: []keyring.BackendType{keyring.FileBackend}, FileDir: identityDir, FilePasswordFunc: func(string) (string, error) { return identityService, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err := ring.Set(keyring.Item{Key: "identity", Data: []byte(`{"sub":"local_x"}`)}); err != nil {
		t.Fatal(err)
	}

	if _, err := DeleteAllTokens(); err != nil {
		t.Fatal(err)
	}
	left, _ := tokens.Keys()
	if len(left) != 0 {
		t.Fatalf("the tokens' ring must be emptied, got %v", left)
	}
	item, err := ring.Get("identity")
	if err != nil || string(item.Data) != `{"sub":"local_x"}` {
		t.Fatalf("the identity must survive the sweep: %v %q", err, item.Data)
	}
	if _, err := os.Stat(identityDir); err != nil {
		t.Fatalf("the identity's directory must survive the sweep: %v", err)
	}
}

/* The identity's config names its own service and directory, with the tokens' backend order. */
func TestIdentityKeychainConfigIsItsOwnServiceAndDirectory(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	cfg := identityKeychainConfig()
	if cfg.ServiceName != "flmnt-identity" || cfg.ServiceName == keychainService {
		t.Fatalf("service %q", cfg.ServiceName)
	}
	if cfg.FileDir != "/home/u/.filament/identity" || cfg.FileDir == keychainConfig().FileDir {
		t.Fatalf("file dir %q", cfg.FileDir)
	}
	if len(cfg.AllowedBackends) != len(keychainConfig().AllowedBackends) {
		t.Fatal("the identity ring uses the tokens' backend order")
	}
	if pw, _ := cfg.FilePasswordFunc(""); pw != "flmnt-identity" {
		t.Fatalf("password %q", pw)
	}
}
