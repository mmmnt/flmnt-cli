// Package identity is the CLI's LOCAL IDENTITY: a sub derived from the host machine's marker and the
// OS user, persisted as a digest, from which per-tool client ids derive. A local flmnt (the compose
// harness) mints credentials for this identity; nothing is reused from the hosted login, nothing is
// generated at random, and only the digest ever leaves the machine. The identity is who this machine
// is, not a login: `flmnt login --local` records it and makes the harness the session, and no logout
// removes it (founder ruling 2026-10-20 — identity and session are separate).
package identity

import (
	"crypto/rand"
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

	"github.com/99designs/keyring"
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

// Identity is what the machine holds: the derived sub and when it was made; for an identity minted
// since QRM-ADR-016 §5g the random salt that makes it unique by construction; and the subs this
// machine answered as before a re-mint, so no one is ever asked which UUID was theirs. The marker and
// the user it derives from are never stored.
type Identity struct {
	Sub       string     `json:"sub"`
	CreatedAt string     `json:"created_at"`
	Salt      string     `json:"salt,omitempty"`
	History   []Replaced `json:"history,omitempty"`
}

// Replaced is a sub this machine used to answer as, and when it stopped.
type Replaced struct {
	Sub        string `json:"sub"`
	CreatedAt  string `json:"created_at"`
	ReplacedAt string `json:"replaced_at"`
}

// Derive is sub = Prefix + UUIDv5(Namespace, marker + "\n" + osUser): the same machine and user derive
// the same sub forever. Identities recorded before the salt keep it; a new login salts (DeriveSalted).
func Derive(marker, osUser string) Identity {
	return Identity{Sub: Prefix + uuid5(Namespace, marker+"\n"+osUser), CreatedAt: time.Now().UTC().Format(time.RFC3339)}
}

// DeriveSalted is sub = Prefix + UUIDv5(Namespace, marker + "\n" + osUser + "\n" + salt): with a random
// salt no two logins collide — not two people on cloned machine ids, not two logins of one person
// (QRM-ADR-016 §5g). Prevention over detection; nothing is kept on the broker.
func DeriveSalted(marker, osUser, salt string) Identity {
	return Identity{Sub: Prefix + uuid5(Namespace, marker+"\n"+osUser+"\n"+salt), Salt: salt, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
}

// NewSalt is 16 random bytes, hex.
func NewSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("identity: no randomness: %v", err))
	}
	return hex.EncodeToString(b)
}

// Remint replaces the identity with a freshly salted one and RECORDS the relationship: the CLI is
// the one party that ever knows both subs at this moment, so the old sub, its lifetime and the new
// sub travel together — to the link request and to a claim — and nobody maps UUIDs by hand.
func Remint(old Identity, marker, osUser string, now time.Time) Identity {
	next := DeriveSalted(marker, osUser, NewSalt())
	next.CreatedAt = now.UTC().Format(time.RFC3339)
	next.History = append(append([]Replaced{}, old.History...), Replaced{Sub: old.Sub, CreatedAt: old.CreatedAt, ReplacedAt: next.CreatedAt})
	return next
}

// Formerly is every sub this machine answered as before the current one, oldest first.
func (i Identity) Formerly() []string {
	out := make([]string, 0, len(i.History))
	for _, h := range i.History {
		out = append(out, h.Sub)
	}
	return out
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

// The identity lives in the OS keychain under its own service (auth.OpenIdentityRing) — the one
// copy that survives `rm -rf ~/.filament`, a logout and a hosted login — and the json file beside the
// config is a cache of it, which is what made the login explicit before the keychain held it.
const ringKey = "identity"

// OpenRing is the identity's keyring; a test points it at an in-memory ring.
var OpenRing = auth.OpenIdentityRing

// StorageDescription names where the identity is kept on this build and platform.
func StorageDescription() string {
	return auth.IdentityStorageDescription()
}

// Save writes the identity to the keychain, one line (the macOS `security` CLI prints a multi-line
// secret back as hex), and refreshes the indented file cache.
func Save(i Identity) error {
	compact, err := json.Marshal(i)
	if err != nil {
		return err
	}
	ring, err := OpenRing()
	if err != nil {
		return err
	}
	if err := ring.Set(keyring.Item{Key: ringKey, Data: compact}); err != nil {
		return err
	}
	indented, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	return writeCache(indented)
}

func writeCache(raw []byte) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Load reads the keychain; an identity recorded by an older build lives only in the file and is
// moved into the keychain on first read, so it is never re-derived and never lost to a logout.
func Load() (Identity, error) {
	ring, err := OpenRing()
	if err != nil {
		return Identity{}, err
	}
	item, err := ring.Get(ringKey)
	if err == nil {
		return parse(item.Data)
	}
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		return Identity{}, err
	}
	cached, err := loadCache()
	if err != nil {
		return Identity{}, err
	}
	id, err := parse(cached)
	if err != nil {
		return Identity{}, err
	}
	compact, err := json.Marshal(id)
	if err != nil {
		return Identity{}, err
	}
	if err := ring.Set(keyring.Item{Key: ringKey, Data: compact}); err != nil {
		return Identity{}, err
	}
	return id, nil
}

func loadCache() ([]byte, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return raw, nil
}

func parse(raw []byte) (Identity, error) {
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
