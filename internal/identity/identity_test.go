package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

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
	t.Setenv("HOME", t.TempDir())
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
	t.Setenv("HOME", t.TempDir())
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
	t.Setenv("HOME", t.TempDir())
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
