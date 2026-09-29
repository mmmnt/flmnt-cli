package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runRoot(t *testing.T, args ...string) string {
	t.Helper()
	boom := &cobra.Command{
		Use:  "boom-probe",
		RunE: func(*cobra.Command, []string) error { return errors.New("the stream holds no DOC-NODE entry") },
	}
	rootCmd.AddCommand(boom)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.RemoveCommand(boom)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	_ = rootCmd.Execute()
	return out.String()
}

/*
A runtime failure has nothing to do with the flags, so printing them buries the reason. `flmnt corpus`

	refusing a workspace with no DOC-NODE answered in one line and then twelve lines of flag help.
*/
func TestARuntimeErrorDoesNotPrintTheFlagList(t *testing.T) {
	out := runRoot(t, "boom-probe")

	if !strings.Contains(out, "the stream holds no DOC-NODE entry") {
		t.Errorf("the reason must still be reported; got\n%s", out)
	}
	if strings.Contains(out, "Usage:") {
		t.Errorf("a runtime error must not print the flag list; got\n%s", out)
	}
}

/* A USAGE error is the case where the flags are the answer, so that one keeps them. */
func TestAnUnknownFlagStillShowsUsage(t *testing.T) {
	out := runRoot(t, "boom-probe", "--no-such-flag")

	if !strings.Contains(out, "Usage:") {
		t.Errorf("an unknown flag is exactly when the flag list helps; got\n%s", out)
	}
}
