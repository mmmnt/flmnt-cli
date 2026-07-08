package auth

import (
	"bytes"
	"errors"
	"os/exec"

	"github.com/99designs/keyring"
)

// securityBin is the macOS keychain CLI. Overridable in tests.
var securityBin = "/usr/bin/security"

// itemNotFoundExit is the exit code `security` returns when a generic-password item does not exist.
const itemNotFoundExit = 44

// securityKeyring implements keyring.Keyring on macOS by shelling out to /usr/bin/security, so
// release builds get the real login Keychain without CGO. The 99designs/keyring KeychainBackend
// needs cgo and is compiled out of our CGO_ENABLED=0 release builds, which otherwise fall through to
// the file backend encrypted with a hardcoded passphrase — no real protection. Windows and Linux keep
// their cgo-free OS backends (WinCred / SecretService) via keyring.Open.
type securityKeyring struct {
	service string
	run     func(args ...string) (stdout []byte, exitCode int, err error)
}

func runSecurity(args ...string) ([]byte, int, error) {
	out, err := exec.Command(securityBin, args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return out, ee.ExitCode(), err
		}
		return out, -1, err
	}
	return out, 0, nil
}

func newSecurityKeyring() securityKeyring {
	return securityKeyring{service: keychainService, run: runSecurity}
}

func (s securityKeyring) Set(item keyring.Item) error {
	_, _, err := s.run("add-generic-password", "-U", "-s", s.service, "-a", item.Key, "-w", string(item.Data))
	return err
}

func (s securityKeyring) Get(key string) (keyring.Item, error) {
	out, code, err := s.run("find-generic-password", "-s", s.service, "-a", key, "-w")
	if code == itemNotFoundExit {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}
	if err != nil {
		return keyring.Item{}, err
	}
	return keyring.Item{Key: key, Data: bytes.TrimRight(out, "\n")}, nil
}

func (s securityKeyring) Remove(key string) error {
	_, code, err := s.run("delete-generic-password", "-s", s.service, "-a", key)
	if code == itemNotFoundExit {
		return keyring.ErrKeyNotFound
	}
	return err
}

// Keys is unused by the token store (it addresses items by known key) and the security CLI has no
// clean per-service enumeration, so it reports none rather than dumping the whole keychain.
func (s securityKeyring) Keys() ([]string, error) { return nil, nil }

// GetMetadata is not exposed by the security CLI; the token store never reads it.
func (s securityKeyring) GetMetadata(string) (keyring.Metadata, error) { return keyring.Metadata{}, nil }
