// Package identity is the CLI's LOCAL IDENTITY: a sub derived from the host machine's marker and the
// OS user, persisted as a digest, from which per-tool client ids derive. A local flmnt (the compose
// harness) mints credentials for this identity; nothing is reused from the hosted login, nothing is
// generated at random, and only the digest ever leaves the machine. The identity is who this machine
// is, not a login: `flmnt login --local` records it and makes the harness the session, and no logout
// removes it (founder ruling 2026-10-20 — identity and session are separate).
package identity

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

// Namespace is the fixed UUIDv5 namespace every local sub derives under. Changing it would change
// every local identity, so it is defined once and documented in the README. Prefix marks both derived
// ids as local (the `conn_` precedent) so the hosted side can tell a local identity without a lookup
// when a person later links this machine to a hosted account.
const (
	Namespace = "c0a3f0d2-5b1e-4d7a-9c3e-2f6b8a1d4e70"
	Prefix    = "local_"
)

var ErrNotFound = errors.New("no local login (run `flmnt login --local`)")

// Identity is what the login file holds: the derived sub and when the login was made. The marker
// and the user it derives from are never stored.
type Identity struct {
	Sub       string `json:"sub"`
	CreatedAt string `json:"created_at"`
}

// Derive is sub = Prefix + UUIDv5(Namespace, marker + "\n" + osUser): the same machine and user derive
// the same sub forever; two people or two machines never collide.
func Derive(marker, osUser string) Identity {
	return Identity{Sub: Prefix + uuid5(Namespace, marker+"\n"+osUser), CreatedAt: time.Now().UTC().Format(time.RFC3339)}
}

// ClientID is Prefix + UUIDv5(sub, tool), the sub's UUID being the namespace: a tool name derives the
// same client id forever for this identity.
func (i Identity) ClientID(tool string) string {
	return Prefix + uuid5(strings.TrimPrefix(i.Sub, Prefix), tool)
}

// OSUser is the account the CLI runs as, the second input of the derivation.
func OSUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	if u.Username == "" {
		return "", errors.New("the OS user has no name")
	}
	return u.Username, nil
}

func Path() (string, error) {
	dir, err := auth.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "local-identity.json"), nil
}

func Save(i Identity) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func Load() (Identity, error) {
	path, err := Path()
	if err != nil {
		return Identity{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Identity{}, ErrNotFound
		}
		return Identity{}, err
	}
	var i Identity
	if err := json.Unmarshal(raw, &i); err != nil {
		return Identity{}, err
	}
	if i.Sub == "" {
		return Identity{}, ErrNotFound
	}
	return i, nil
}

// uuid5 is RFC 4122 name-based UUID generation with SHA-1.
func uuid5(namespace, name string) string {
	ns, err := hex.DecodeString(strings.ReplaceAll(namespace, "-", ""))
	if err != nil || len(ns) != 16 {
		panic(fmt.Sprintf("identity: namespace %q is not a UUID", namespace))
	}
	h := sha1.New()
	h.Write(ns)
	h.Write([]byte(name))
	sum := h.Sum(nil)[:16]
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	s := hex.EncodeToString(sum)
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}
