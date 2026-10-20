package auth

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
	"unicode/utf8"

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
	return keyring.Item{Key: key, Data: decodeSecurityValue(bytes.TrimRight(out, "\n"))}, nil
}

// decodeSecurityValue undoes `security -w`'s one quirk: a secret that holds a newline or a byte outside
// printable ASCII is printed as hex rather than as itself (the identity's JSON, for one). A value that
// is wholly hex of even length and decodes to text is that text; anything else is returned as read.
func decodeSecurityValue(out []byte) []byte {
	if len(out) == 0 || len(out)%2 != 0 {
		return out
	}
	for _, c := range out {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return out
		}
	}
	decoded, err := hex.DecodeString(string(out))
	if err != nil || !utf8.Valid(decoded) {
		return out
	}
	return decoded
}

func (s securityKeyring) Remove(key string) error {
	_, code, err := s.run("delete-generic-password", "-s", s.service, "-a", key)
	if code == itemNotFoundExit {
		return keyring.ErrKeyNotFound
	}
	return err
}

// Keys lists every item stored under the CLI's service. The security CLI has no per-service
// enumeration, so the keychain's metadata is dumped (`dump-keychain` without -d prints attributes,
// never secrets, and never prompts) and the items whose service is ours are picked out. Logout
// relies on this to leave nothing behind, whatever server a token was stored for.
func (s securityKeyring) Keys() ([]string, error) {
	out, _, err := s.run("dump-keychain")
	if err != nil {
		return nil, err
	}
	return parseSecurityDump(out, s.service), nil
}

// parseSecurityDump reads `security dump-keychain` output: items are separated by a `keychain:`
// line, and within an item the "acct" attribute (our key) precedes the "svce" attribute (the
// service). The account of every item whose service matches is returned, in dump order.
func parseSecurityDump(dump []byte, service string) []string {
	var keys []string
	acct, found := "", false
	flush := func() {
		if found && acct != "" {
			keys = append(keys, acct)
		}
		acct, found = "", false
	}
	for _, line := range strings.Split(string(dump), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "keychain:"):
			flush()
		case strings.HasPrefix(trimmed, `"acct"<blob>="`):
			acct = strings.TrimSuffix(strings.TrimPrefix(trimmed, `"acct"<blob>="`), `"`)
		case trimmed == `"svce"<blob>="`+service+`"`:
			found = true
		}
	}
	flush()
	return keys
}

// GetMetadata is not exposed by the security CLI; the token store never reads it.
func (s securityKeyring) GetMetadata(string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}
