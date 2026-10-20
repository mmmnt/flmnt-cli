package identity

import (
	"os"
	"path/filepath"
	"testing"
)

/* RFC 4122 appendix vector for UUIDv5: the DNS namespace + "python.org". */
func TestUUID5MatchesTheRFCVector(t *testing.T) {
	if got := uuid5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "python.org"); got != "886313e1-3b8a-5372-9b90-0c9aee199e5d" {
		t.Fatalf("got %s", got)
	}
}

/*
The sub is derived, never generated: the same machine and email give the same sub forever, a tool

	name gives the same client id forever, and two people or two machines never collide — which is
	what keeps identities apart behind a shared proxy. Nothing comes from the hosted login.
*/
func TestSubAndClientIDAreDeterministicAndDistinct(t *testing.T) {
	a := Identity{Email: "mike@flmnt.ai", Marker: "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"}
	again := Identity{Email: "mike@flmnt.ai", Marker: "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"}
	otherPerson := Identity{Email: "howie@flmnt.ai", Marker: a.Marker}
	otherMachine := Identity{Email: a.Email, Marker: "00000000-0000-0000-0000-000000000001"}

	if a.Sub() != again.Sub() {
		t.Fatal("the same machine and email must derive the same sub")
	}
	if a.Sub() == otherPerson.Sub() || a.Sub() == otherMachine.Sub() {
		t.Fatal("different people or machines must not share a sub")
	}
	if a.ClientID("flmnt-cli") != again.ClientID("flmnt-cli") {
		t.Fatal("a tool name must derive the same client id forever")
	}
	if a.ClientID("flmnt-cli") == a.ClientID("claude-code-local") || a.ClientID("flmnt-cli") == otherPerson.ClientID("flmnt-cli") {
		t.Fatal("client ids are per tool and per person")
	}
	if a.Sub() != uuid5(Namespace, a.Marker+"\n"+a.Email) || a.ClientID("flmnt-cli") != uuid5(a.Sub(), "flmnt-cli") {
		t.Fatal("derivation must be exactly the documented one")
	}
}

func TestSaveAndLoadRoundTripWithOwnerOnlyPermissions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := Identity{Email: "mike@flmnt.ai", Marker: "ABB0B3CF"}

	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || got != want {
		t.Fatalf("got %+v err %v", got, err)
	}
	path, _ := Path()
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
	}
	if filepath.Base(path) != "local-identity.json" {
		t.Fatalf("path %s", path)
	}
}

func TestLoadReportsNoLocalLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := Load(); err != ErrNotFound {
		t.Fatalf("err %v, want ErrNotFound", err)
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
