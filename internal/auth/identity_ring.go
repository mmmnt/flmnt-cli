package auth

import (
	"path/filepath"

	"github.com/99designs/keyring"
)

// identityService is the keychain service the machine's LOCAL IDENTITY lives under (QRM-ADR-016
// §5f–§5g). It is deliberately not the tokens' service: DeleteAllTokens enumerates and removes every
// item under keychainService and removes ~/.filament/keyring whole, and the identity must survive
// every logout and every login of the other kind. On the file backend it lives in its own directory
// for the same reason.
const identityService = "flmnt-identity"

func identityKeychainConfig() keyring.Config {
	fileDir := ""
	if dir, err := ConfigDir(); err == nil {
		fileDir = filepath.Join(dir, "identity")
	}
	return keyring.Config{
		ServiceName:      identityService,
		AllowedBackends:  keychainConfig().AllowedBackends,
		FileDir:          fileDir,
		FilePasswordFunc: func(string) (string, error) { return identityService, nil },
	}
}

// OpenIdentityRing is the identity's keyring: the macOS Keychain through the `security` CLI where
// that is the token path too, else the platform backend keyring.Open selects, never the tokens' items.
var OpenIdentityRing = func() (keyring.Keyring, error) {
	if usingSecurityKeychain() {
		return securityKeyring{service: identityService, run: runSecurity}, nil
	}
	return keyring.Open(identityKeychainConfig())
}

// IdentityStorageDescription names where the identity is kept: the OS keychain, or its own encrypted
// file directory on a build or platform without one.
func IdentityStorageDescription() string {
	if usingSecurityKeychain() {
		return "the macOS Keychain (service flmnt-identity)"
	}
	cfg := identityKeychainConfig()
	if effectiveBackend() == keyring.FileBackend {
		return "an encrypted file in " + cfg.FileDir
	}
	return "the OS keychain (service flmnt-identity)"
}
