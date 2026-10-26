package auth

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/99designs/keyring"
)

/*
One login at a time: a logout, or a login of the other kind, clears every token the CLI ever
stored, not only the one for the server the config happens to name. The staging tokens of a
machine that moved to production, the entries a `?workspace=` URL variant once split off, the
legacy single blob — all of it goes, and the encrypted file keyring of an older build with it.
*/
func TestDeleteAllTokensClearsEveryServerAndTheFileKeyring(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacyDir := filepath.Join(home, ".flmnt", "keyring")
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "https:%2F%2Fmcp.staging.flmnt.dev|access_token"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ring := fileRing(t)
	for _, key := range []string{
		"https://mcp.production.flmnt.ai|access_token",
		"https://mcp.production.flmnt.ai|refresh_token",
		"https://mcp.production.flmnt.ai|id_token",
		"https://mcp.staging.flmnt.dev|access_token",
		"https://mcp.staging.flmnt.dev", // legacy single blob
	} {
		if err := ring.Set(keyring.Item{Key: key, Data: []byte("t")}); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := DeleteAllTokens()
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 5 {
		t.Errorf("five keys were stored; removed %v", removed)
	}
	left, _ := ring.Keys()
	if len(left) != 0 {
		t.Errorf("nothing may remain; got %v", left)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Error("the legacy file keyring under ~/.flmnt must be removed whole")
	}
	if _, err := LoadToken("https://mcp.production.flmnt.ai"); err != ErrNotFound {
		t.Errorf("after DeleteAllTokens, LoadToken = %v, want ErrNotFound", err)
	}
}

/*
The sweep is total for CREDENTIALS and touches nothing else under ~/.flmnt: the local identity
file beside the config survives every logout (founder ruling 2026-10-20 — identity and session are
separate; a keychain-resident identity will likewise live under its own service, never this one).
*/
func TestDeleteAllTokensLeavesTheLocalIdentityBesideTheConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	identityPath := filepath.Join(home, ".flmnt", "local-identity.json")
	if err := os.MkdirAll(filepath.Dir(identityPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identityPath, []byte(`{"sub":"local_x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ring := fileRing(t)
	if err := ring.Set(keyring.Item{Key: "https://mcp.production.flmnt.ai|access_token", Data: []byte("t")}); err != nil {
		t.Fatal(err)
	}

	if _, err := DeleteAllTokens(); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(identityPath); err != nil || string(raw) != `{"sub":"local_x"}` {
		t.Fatalf("the identity file must be untouched; got %q %v", raw, err)
	}
}

func TestOriginsOfCollapsesFieldsAndLegacyBlobs(t *testing.T) {
	got := OriginsOf([]string{
		"https://a|access_token", "https://a|refresh_token", "https://a|id_token",
		"https://b", "https://b|access_token", "", "https://a",
	})
	if want := []string{"https://a", "https://b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

/* The macOS keychain is enumerated from dump-keychain metadata: ours by service, the account is the key. */
func TestSecurityKeyringKeysPicksOurServiceOutOfTheDump(t *testing.T) {
	dump := `keychain: "/Users/u/Library/Keychains/login.keychain-db"
version: 512
class: "genp"
attributes:
    0x00000007 <blob>="quorum-cli"
    "acct"<blob>="https://mcp.production.flmnt.ai|access_token"
    "cdat"<timedate>=0x32303236  "20260721023016Z\000"
    "svce"<blob>="quorum-cli"
    "type"<uint32>=<NULL>
keychain: "/Users/u/Library/Keychains/login.keychain-db"
version: 512
class: "genp"
attributes:
    "acct"<blob>="someone@example.com"
    "svce"<blob>="com.apple.something"
keychain: "/Users/u/Library/Keychains/login.keychain-db"
version: 512
class: "genp"
attributes:
    "acct"<blob>="https://mcp.staging.flmnt.dev"
    "svce"<blob>="quorum-cli"
`
	var args []string
	k := securityKeyring{service: "quorum-cli", run: func(a ...string) ([]byte, int, error) {
		args = a
		return []byte(dump), 0, nil
	}}
	got, err := k.Keys()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"dump-keychain"}) {
		t.Errorf("metadata only — never -d; got %v", args)
	}
	want := []string{"https://mcp.production.flmnt.ai|access_token", "https://mcp.staging.flmnt.dev"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestConfigLoginKindInfersHostedForAnOlderConfig(t *testing.T) {
	if k := (CLIConfig{ServerURL: "https://mcp.production.flmnt.ai"}).LoginKind(); k != LoginHosted {
		t.Errorf("an older config with a server_url was written by `flmnt login`; kind = %q", k)
	}
	if k := (CLIConfig{Login: LoginLocal, ServerURL: "http://localhost:8000"}).LoginKind(); k != LoginLocal {
		t.Errorf("kind = %q", k)
	}
	if (CLIConfig{}).Session() {
		t.Error("an empty config is no session")
	}
}
