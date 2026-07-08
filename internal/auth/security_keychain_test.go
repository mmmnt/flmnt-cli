package auth

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/99designs/keyring"
)

func fakeRun(out []byte, code int, err error) (func(...string) ([]byte, int, error), *[]string) {
	var got []string
	return func(args ...string) ([]byte, int, error) {
		got = args
		return out, code, err
	}, &got
}

func TestSecurityKeyringSetAddsGenericPasswordWithUpsert(t *testing.T) {
	run, got := fakeRun(nil, 0, nil)
	k := securityKeyring{service: "svc", run: run}
	if err := k.Set(keyring.Item{Key: "k1", Data: []byte("secret")}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := []string{"add-generic-password", "-U", "-s", "svc", "-a", "k1", "-w", "secret"}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("args = %v, want %v", *got, want)
	}
}

func TestSecurityKeyringGetTrimsTrailingNewline(t *testing.T) {
	run, got := fakeRun([]byte("secret\n"), 0, nil)
	k := securityKeyring{service: "svc", run: run}
	item, err := k.Get("k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(item.Data) != "secret" {
		t.Fatalf("data = %q, want %q", item.Data, "secret")
	}
	if item.Key != "k1" {
		t.Fatalf("key = %q", item.Key)
	}
	want := []string{"find-generic-password", "-s", "svc", "-a", "k1", "-w"}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("args = %v, want %v", *got, want)
	}
}

func TestSecurityKeyringGetMapsMissingItemToErrKeyNotFound(t *testing.T) {
	run, _ := fakeRun(nil, itemNotFoundExit, errors.New("exit 44"))
	k := securityKeyring{service: "svc", run: run}
	if _, err := k.Get("missing"); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("err = %v, want ErrKeyNotFound", err)
	}
}

func TestSecurityKeyringGetPropagatesOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	run, _ := fakeRun(nil, 1, boom)
	k := securityKeyring{service: "svc", run: run}
	if _, err := k.Get("k1"); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestSecurityKeyringRemoveDeletesGenericPassword(t *testing.T) {
	run, got := fakeRun(nil, 0, nil)
	k := securityKeyring{service: "svc", run: run}
	if err := k.Remove("k1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	want := []string{"delete-generic-password", "-s", "svc", "-a", "k1"}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("args = %v, want %v", *got, want)
	}
}

func TestSecurityKeyringRemoveMapsMissingItemToErrKeyNotFound(t *testing.T) {
	run, _ := fakeRun(nil, itemNotFoundExit, errors.New("exit 44"))
	k := securityKeyring{service: "svc", run: run}
	if err := k.Remove("missing"); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("err = %v, want ErrKeyNotFound", err)
	}
}

func TestSecurityKeyringRemovePropagatesOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	run, _ := fakeRun(nil, 1, boom)
	k := securityKeyring{service: "svc", run: run}
	if err := k.Remove("k1"); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestSecurityKeyringKeysAndMetadataAreEmpty(t *testing.T) {
	k := securityKeyring{service: "svc"}
	keys, err := k.Keys()
	if err != nil || keys != nil {
		t.Fatalf("Keys() = %v, %v", keys, err)
	}
	md, err := k.GetMetadata("k1")
	if err != nil || md != (keyring.Metadata{}) {
		t.Fatalf("GetMetadata() = %v, %v", md, err)
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-security")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return p
}

func TestRunSecurityReturnsStdoutOnSuccess(t *testing.T) {
	orig := securityBin
	t.Cleanup(func() { securityBin = orig })
	securityBin = writeScript(t, "printf hello; exit 0")
	out, code, err := runSecurity("whatever")
	if err != nil || code != 0 || string(out) != "hello" {
		t.Fatalf("runSecurity = %q, %d, %v", out, code, err)
	}
}

func TestRunSecurityExtractsNonZeroExitCode(t *testing.T) {
	orig := securityBin
	t.Cleanup(func() { securityBin = orig })
	securityBin = writeScript(t, "exit 44")
	_, code, err := runSecurity("x")
	if code != itemNotFoundExit || err == nil {
		t.Fatalf("code = %d, err = %v, want 44 + error", code, err)
	}
}

func TestRunSecurityReportsStartFailureAsMinusOne(t *testing.T) {
	orig := securityBin
	t.Cleanup(func() { securityBin = orig })
	securityBin = filepath.Join(t.TempDir(), "does-not-exist")
	_, code, err := runSecurity("x")
	if code != -1 || err == nil {
		t.Fatalf("code = %d, err = %v, want -1 + error", code, err)
	}
}

func forceKeychain(t *testing.T, goos string, binExists bool) {
	t.Helper()
	origGoos, origBin := goosForKeychain, securityBin
	t.Cleanup(func() { goosForKeychain, securityBin = origGoos, origBin })
	goosForKeychain = goos
	if binExists {
		securityBin = writeScript(t, "exit 0")
	} else {
		securityBin = filepath.Join(t.TempDir(), "missing")
	}
}

func TestUsingSecurityKeychainTrueOnDarwinWithBinary(t *testing.T) {
	forceKeychain(t, "darwin", true)
	if !usingSecurityKeychain() {
		t.Fatal("want true on darwin with security present")
	}
}

func TestUsingSecurityKeychainFalseOffDarwin(t *testing.T) {
	forceKeychain(t, "linux", true)
	if usingSecurityKeychain() {
		t.Fatal("want false off darwin")
	}
}

func TestUsingSecurityKeychainFalseWhenBinaryMissing(t *testing.T) {
	forceKeychain(t, "darwin", false)
	if usingSecurityKeychain() {
		t.Fatal("want false when security binary is absent")
	}
}

func TestStorageDescriptionNamesMacKeychain(t *testing.T) {
	forceKeychain(t, "darwin", true)
	if got := StorageDescription(); got != "the macOS Keychain" {
		t.Fatalf("StorageDescription = %q", got)
	}
}

func TestStorageDescriptionOffDarwinDescribesTheKeyringBackend(t *testing.T) {
	forceKeychain(t, "linux", true)
	got := StorageDescription()
	if got == "" || got == "the macOS Keychain" {
		t.Fatalf("off-darwin description = %q, want the keyring-backend phrasing", got)
	}
}

func TestOpenKeyringSelectsSecurityBackendOnDarwin(t *testing.T) {
	forceKeychain(t, "darwin", true)
	ring, err := openKeyring()
	if err != nil {
		t.Fatalf("openKeyring: %v", err)
	}
	if _, ok := ring.(securityKeyring); !ok {
		t.Fatalf("openKeyring returned %T, want securityKeyring", ring)
	}
}
