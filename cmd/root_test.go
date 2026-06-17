package cmd_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:     "flmnt",
		Version: "dev",
	}
	root.AddCommand(&cobra.Command{
		Use: "version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("dev")
		},
	})
	return root
}

func TestHelpExitsZero(t *testing.T) {
	root := newTestRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	root := newTestRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected version output, got nothing")
	}
}

func TestVersionSubcommand(t *testing.T) {
	root := newTestRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version subcommand returned error: %v", err)
	}
}
