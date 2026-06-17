package auth

import (
	"testing"

	"github.com/99designs/keyring"
)

func TestKeychainConfigForbidsInsecureBackends(t *testing.T) {
	cfg := keychainConfig()
	if len(cfg.AllowedBackends) == 0 {
		t.Fatal("AllowedBackends must be set to restrict to OS keychain backends only")
	}
	forbidden := map[keyring.BackendType]bool{
		keyring.FileBackend:  true,
		keyring.PassBackend:  true,
		keyring.KeyCtlBackend: true,
		keyring.KWalletBackend: true,
	}
	for _, b := range cfg.AllowedBackends {
		if forbidden[b] {
			t.Errorf("insecure backend %q must not be in AllowedBackends", b)
		}
	}
}
