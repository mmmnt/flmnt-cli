package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"github.com/mmmnt/flmnt-cli/internal/auth"
)

// memoryRing gives a test its own in-memory keyring and a fresh HOME, so nothing reaches the real keychain.
func memoryRing(t *testing.T) keyring.Keyring {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	ring := keyring.NewArrayKeyring(nil)
	orig := OpenRing
	OpenRing = func() (keyring.Keyring, error) { return ring, nil }
	t.Cleanup(func() { OpenRing = orig })
	return ring
}

/* RFC 4122 appendix vector for UUIDv5: the DNS namespace + "python.org". */
func TestUUID5MatchesTheRFCVector(t *testing.T) {
	if got := uuid5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "python.org"); got != "886313e1-3b8a-5372-9b90-0c9aee199e5d" {
		t.Fatalf("got %s", got)
	}
}

/*
The sub is derived, never generated: the same machine and OS user give the same sub forever, a tool

	name gives the same client id forever, and two people or two machines never collide — which is
	what keeps identities apart behind a shared proxy. Nothing comes from the hosted login.
*/
func TestSubAndClientIDAreDeterministicAndDistinct(t *testing.T) {
	marker := "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"
	a, again := Derive(marker, "mike"), Derive(marker, "mike")
	otherPerson, otherMachine := Derive(marker, "howie"), Derive("00000000-0000-0000-0000-000000000001", "mike")

	if a.Sub != again.Sub {
		t.Fatal("the same machine and user must derive the same sub")
	}
	if a.Sub == otherPerson.Sub || a.Sub == otherMachine.Sub {
		t.Fatal("different people or machines must not share a sub")
	}
	if a.ClientID("flmnt-cli") != again.ClientID("flmnt-cli") {
		t.Fatal("a tool name must derive the same client id forever")
	}
	if a.ClientID("flmnt-cli") == a.ClientID("claude-code-local") || a.ClientID("flmnt-cli") == otherPerson.ClientID("flmnt-cli") {
		t.Fatal("client ids are per tool and per person")
	}
	if a.Sub != "local_"+uuid5(Namespace, marker+"\n"+"mike") || a.ClientID("flmnt-cli") != "local_"+uuid5(strings.TrimPrefix(a.Sub, "local_"), "flmnt-cli") {
		t.Fatal("derivation must be exactly the documented one")
	}
	if !strings.HasPrefix(a.Sub, "local_") || !strings.HasPrefix(a.ClientID("flmnt-cli"), "local_") {
		t.Fatal("both derived ids carry the local_ prefix so the hosted side recognises them without a lookup")
	}
}

/* Only the digest is kept: the file that makes the login explicit holds the sub and when, never the raw marker. */
func TestSaveKeepsTheDigestNotTheMarker(t *testing.T) {
	memoryRing(t)
	marker := "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"
	id := Derive(marker, "mike")

	if err := Save(id); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || got.Sub != id.Sub || got.CreatedAt == "" {
		t.Fatalf("got %+v err %v", got, err)
	}
	path, _ := Path()
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), marker) || strings.Contains(string(raw), "mike") {
		t.Fatalf("the raw marker or user must never be stored: %s", raw)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
	}
	if filepath.Base(path) != "local-identity.json" {
		t.Fatalf("path %s", path)
	}
}

/* The identity lives beside config.json, never inside ~/.filament/keyring, which a logout removes whole. */
func TestPathIsBesideTheConfigNotInsideTheKeyring(t *testing.T) {
	memoryRing(t)
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := auth.ConfigDir()
	if filepath.Dir(path) != dir || strings.Contains(path, "keyring") {
		t.Fatalf("path %s must be directly under %s", path, dir)
	}
}

func TestLoadReportsNoLocalLogin(t *testing.T) {
	memoryRing(t)
	if _, err := Load(); err != ErrNotFound {
		t.Fatalf("err %v, want ErrNotFound", err)
	}
	if err := Save(Derive("m", "u")); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(); err != nil || got.Sub != Derive("m", "u").Sub {
		t.Fatalf("after Save: %+v %v", got, err)
	}
}

func TestMarkerParsers(t *testing.T) {
	ioreg := "+-o MacBookPro18,3  <class IOPlatformExpertDevice, id 0x100000111, registered, matched, active, busy 0 (90 ms), retain 48>\n" +
		"    {\n      \"IOPlatformUUID\" = \"ABB0B3CF-5122-5F8A-835B-A01323C6EDCD\"\n      \"IOPlatformSerialNumber\" = \"X\"\n    }\n"
	if got := parseIORegUUID(ioreg); got != "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD" {
		t.Errorf("ioreg: %q", got)
	}
	reg := "\r\nHKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Cryptography\r\n    MachineGuid    REG_SZ    4c4c4544-0046-3010-8035-b4c04f4d5a31\r\n\r\n"
	if got := parseRegQuery(reg); got != "4c4c4544-0046-3010-8035-b4c04f4d5a31" {
		t.Errorf("reg: %q", got)
	}
	if parseIORegUUID("nothing here") != "" || parseRegQuery("nothing here") != "" {
		t.Error("absent markers must read as empty, never invented")
	}
}

/*
The identity lives in the OS keychain under its own service (QRM-ADR-016 §5f–§5g): Save writes it

	there and caches it in the file; Load reads the keychain, so a deleted file changes nothing; an
	identity an older build left only in the file is moved into the keychain on first read.
*/
func TestIdentityLivesInItsKeyringAndTheFileIsACache(t *testing.T) {
	ring := memoryRing(t)
	id := DeriveSalted("ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", "mike", "0123456789abcdef0123456789abcdef")
	if err := Save(id); err != nil {
		t.Fatal(err)
	}
	item, err := ring.Get("identity")
	if err != nil || !strings.Contains(string(item.Data), id.Sub) || !strings.Contains(string(item.Data), id.Salt) {
		t.Fatalf("the keychain holds the identity with its salt: %v %s", err, item.Data)
	}
	path, _ := Path()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || got.Sub != id.Sub || got.Salt != id.Salt {
		t.Fatalf("a deleted cache changes nothing: %+v %v", got, err)
	}

	older := memoryRing(t)
	legacy := Derive("ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", "mike")
	cachePath, _ := Path()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte(`{"sub":"`+legacy.Sub+`","created_at":"2026-10-20T16:49:09Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = Load()
	if err != nil || got.Sub != legacy.Sub || got.Salt != "" {
		t.Fatalf("an older identity is read from its file as it is: %+v %v", got, err)
	}
	if item, err := older.Get("identity"); err != nil || !strings.Contains(string(item.Data), legacy.Sub) {
		t.Fatalf("and moved into the keychain: %v", err)
	}
}

/* A salted identity never collides: the same machine and user with another salt is another sub; the same salt is the same sub. */
func TestSaltedDerivationIsUniquePerSaltAndStable(t *testing.T) {
	marker := "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"
	a := DeriveSalted(marker, "mike", "aa")
	if a.Sub != DeriveSalted(marker, "mike", "aa").Sub || a.Sub == DeriveSalted(marker, "mike", "bb").Sub || a.Sub == Derive(marker, "mike").Sub {
		t.Fatal("the salt must change the sub, and only the salt")
	}
	if a.Sub != "local_"+uuid5(Namespace, marker+"\n"+"mike"+"\n"+"aa") {
		t.Fatal("derivation must be exactly the documented one")
	}
	s1, s2 := NewSalt(), NewSalt()
	if len(s1) != 32 || s1 == s2 {
		t.Fatalf("salts are 16 random bytes in hex: %q %q", s1, s2)
	}
}

/* A re-mint keeps the whole history, oldest first, with the lifetime of each sub it replaced. */
func TestRemintRecordsEverySubTheMachineAnsweredAs(t *testing.T) {
	marker := "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"
	first := Derive(marker, "mike")
	first.CreatedAt = "2026-10-20T16:49:09Z"
	second := Remint(first, marker, "mike", time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC))
	third := Remint(second, marker, "mike", time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC))
	if second.Sub == first.Sub || third.Sub == second.Sub || third.Salt == "" || third.Salt == second.Salt {
		t.Fatal("each re-mint is a new salted sub")
	}
	if len(third.History) != 2 || third.History[0].Sub != first.Sub || third.History[0].ReplacedAt != "2026-10-21T09:00:00Z" || third.History[1].Sub != second.Sub || third.History[1].CreatedAt != "2026-10-21T09:00:00Z" {
		t.Fatalf("history %+v", third.History)
	}
	if got := third.Formerly(); len(got) != 2 || got[0] != first.Sub || got[1] != second.Sub {
		t.Fatalf("formerly %v", got)
	}
}
