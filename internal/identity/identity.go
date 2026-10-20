// Package identity is the CLI's LOCAL LOGIN: a declared email plus an identifying marker read from
// the host machine, persisted, from which a unique sub and per-tool client ids derive. A local
// flmnt (the compose harness) mints credentials for this identity; nothing is reused from the hosted
// login or from stream data, and nothing is ever generated at random.
package identity

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

// Namespace is the fixed UUIDv5 namespace every local sub derives under. Changing it would change
// every local identity, so it is defined once and documented in the README.
const Namespace = "c0a3f0d2-5b1e-4d7a-9c3e-2f6b8a1d4e70"

var ErrNotFound = errors.New("no local login (run `flmnt login --local --email <email>`)")

type Identity struct {
	Email  string `json:"email"`
	Marker string `json:"marker"`
}

// Sub is UUIDv5(Namespace, marker + "\n" + email): the same machine and email derive the same sub
// forever; two people or two machines never collide.
func (i Identity) Sub() string {
	return uuid5(Namespace, i.Marker+"\n"+i.Email)
}

// ClientID is UUIDv5(sub, tool): a tool name derives the same client id forever for this person.
func (i Identity) ClientID(tool string) string {
	return uuid5(i.Sub(), tool)
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
	if i.Email == "" || i.Marker == "" {
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
