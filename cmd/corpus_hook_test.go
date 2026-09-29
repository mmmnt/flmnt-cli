package cmd

import (
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

func corpusTestCmd() *cobra.Command {
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")
	c.Flags().String("remote-url", "", "")
	c.Flags().String("project", "", "")
	c.Flags().String("out", "training", "")
	c.Flags().Bool("hook", false, "")
	return c
}

/*
The SessionStart refresh runs before anybody has typed anything, so it must never break a session:

	offline, not logged in, or pointed at a workspace with no DOC-NODE, it leaves the files alone and
	says nothing — the way `brief` and `gate` already behave in hooks.
*/
func TestCorpusUnderHookFailsQuiet(t *testing.T) {
	orig := authHeaderLoadConfig
	defer func() { authHeaderLoadConfig = orig }()
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())

	c := corpusTestCmd()
	if err := c.Flags().Set("hook", "true"); err != nil {
		t.Fatal(err)
	}
	if err := runCorpus(c, nil); err != nil {
		t.Errorf("a hook must not fail a session start; got %v", err)
	}
}

/* Run by hand, the same condition must say so rather than exit silently claiming success. */
func TestCorpusRunByHandReportsWhyItCannotRender(t *testing.T) {
	orig := authHeaderLoadConfig
	defer func() { authHeaderLoadConfig = orig }()
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())

	if err := runCorpus(corpusTestCmd(), nil); err == nil {
		t.Error("without --hook the command must report that it has no server URL")
	}
}
