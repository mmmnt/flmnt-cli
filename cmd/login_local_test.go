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
	t.Cleanup(func() {
		_ = loginCmd.Flags().Set("local", "false")
		_ = loginCmd.Flags().Set("email", "")
	})
	err = rootCmd.Execute()
	return out.String(), err
}

/*
A local flmnt mints for a declared person. The declaration is a LOCAL LOGIN the CLI owns: the email

	the person gives plus the marker the host machine already carries, persisted; the sub and the
	per-tool client ids derive from those two and from nothing else. Founder ruling 2026-10-19.
*/
func TestLoginLocalRecordsTheEmailAndTheMachineMarker(t *testing.T) {
	origMarker, origSave := identityMarker, identitySave
	identityMarker = func() (string, error) { return "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD", nil }
	var saved identity.Identity
	identitySave = func(i identity.Identity) error { saved = i; return nil }
	t.Cleanup(func() { identityMarker, identitySave = origMarker, origSave })

	stdout, err := runLogin(t, "--local", "--email", "mike@flmnt.ai")
	if err != nil {
		t.Fatal(err)
	}
	want := identity.Identity{Email: "mike@flmnt.ai", Marker: "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"}
	if saved != want {
		t.Fatalf("saved %+v, want %+v", saved, want)
	}
	if !strings.Contains(stdout, want.Sub()) {
		t.Fatalf("the derived sub must be shown; got %q", stdout)
	}
}

/* Nothing is ever invented: no email, or a machine with no marker, is a refusal and nothing is written. */
func TestLoginLocalRefusesWithoutAnEmailOrAMarker(t *testing.T) {
	origMarker, origSave := identityMarker, identitySave
	identityMarker = func() (string, error) { return "", errors.New("no machine marker could be read from this host") }
	saves := 0
	identitySave = func(identity.Identity) error { saves++; return nil }
	t.Cleanup(func() { identityMarker, identitySave = origMarker, origSave })

	if _, err := runLogin(t, "--local"); err == nil || !strings.Contains(err.Error(), "--email") {
		t.Fatalf("without --email: %v", err)
	}
	if _, err := runLogin(t, "--local", "--email", "mike@flmnt.ai"); err == nil || !strings.Contains(err.Error(), "marker") {
		t.Fatalf("without a marker: %v", err)
	}
	if saves != 0 {
		t.Fatal("a refused login must write nothing")
	}
}
