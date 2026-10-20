package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/identity"
)

func runLogin(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"login"}, args...))
	t.Cleanup(func() { _ = loginCmd.Flags().Set("local", "false") })
	err = rootCmd.Execute()
	return out.String(), err
}

/*
A local flmnt mints for a declared person. The declaration is a LOCAL LOGIN the CLI owns, with no

	input: the marker the host machine already carries and the OS user, digested to a sub that is
	persisted — the file is what makes the login explicit. Founder ruling 2026-10-19.
*/
func TestLoginLocalDerivesAndRecordsTheSub(t *testing.T) {
	origMarker, origUser, origSave := identityMarker, identityOSUser, identitySave
	identityMarker = func() (string, error) { return "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", nil }
	identityOSUser = func() (string, error) { return "mike", nil }
	var saved identity.Identity
	identitySave = func(i identity.Identity) error { saved = i; return nil }
	t.Cleanup(func() { identityMarker, identityOSUser, identitySave = origMarker, origUser, origSave })

	stdout, err := runLogin(t, "--local")
	if err != nil {
		t.Fatal(err)
	}
	want := identity.Derive("ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", "mike").Sub
	if saved.Sub != want || saved.CreatedAt == "" {
		t.Fatalf("saved %+v, want sub %s", saved, want)
	}
	if !strings.Contains(stdout, want) {
		t.Fatalf("the derived sub must be printed; got %q", stdout)
	}
}

/* Nothing is ever invented: a machine with no marker is a refusal and nothing is written. */
func TestLoginLocalRefusesWithoutAMarker(t *testing.T) {
	origMarker, origUser, origSave := identityMarker, identityOSUser, identitySave
	identityMarker = func() (string, error) { return "", errors.New("no machine marker could be read from this host") }
	identityOSUser = func() (string, error) { return "mike", nil }
	saves := 0
	identitySave = func(identity.Identity) error { saves++; return nil }
	t.Cleanup(func() { identityMarker, identityOSUser, identitySave = origMarker, origUser, origSave })

	if _, err := runLogin(t, "--local"); err == nil || !strings.Contains(err.Error(), "marker") {
		t.Fatalf("without a marker: %v", err)
	}
	if saves != 0 {
		t.Fatal("a refused login must write nothing")
	}
}

func TestLogoutLocalRemovesTheLocalLogin(t *testing.T) {
	orig := identityRemove
	removed := false
	identityRemove = func() error { removed = true; return nil }
	t.Cleanup(func() { identityRemove = orig; _ = logoutCmd.Flags().Set("local", "false") })
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"logout", "--local"})

	if err := rootCmd.Execute(); err != nil || !removed {
		t.Fatalf("err %v removed %v", err, removed)
	}
}
