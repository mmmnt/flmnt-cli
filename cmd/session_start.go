package cmd

import (
	"fmt"
	"os"

	"github.com/mmmnt/flmnt-cli/internal/env"
	"github.com/mmmnt/flmnt-cli/internal/session"
	"github.com/spf13/cobra"
)

var sessionStartCmd = &cobra.Command{
	Use:   "session-start",
	Short: "SessionStart hook: make sure this session's keyframe stream exists and name it",
	Long: "For an agent's SessionStart hook (installed by `flmnt setup`): reads the hook JSON on stdin (session_id,\n" +
		"cwd), resolves the project the way gate does, makes sure {project}::{session_id}::keyframe exists (creating\n" +
		"it when absent) and prints the stream_id the agent passes to write_keyframe and read_keyframe this session.\n" +
		"It never blocks a session: when no flmnt answers, or its endpoint answers\n" +
		"with an error, it says FLMNT_NOT_CONNECTED on stderr and exits 0.",
	RunE: runSessionStart,
}

func runSessionStart(cmd *cobra.Command, args []string) error {
	in := readHookInput(cmd.InOrStdin())
	if in.SessionID == "" {
		return nil // no session named on stdin, so there is no session stream to make sure of
	}
	if in.Cwd != "" {
		if err := os.Chdir(in.Cwd); err != nil {
			return nil
		}
	}
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return nil // not configured — stay quiet
	}
	cwd, _ := os.Getwd()
	project := resolveProject(cmd, cwd)
	if project == "" {
		project = env.Get("PROJECT_ID")
	}
	if project == "" {
		return nil
	}
	gql, err := graphQLClientFor(cmd, serverURL)
	if err != nil {
		return nil
	}
	id, err := session.Ensure(gql, project, in.SessionID)
	if err != nil {
		reportNotConnected(cmd.ErrOrStderr(), err)
		return nil // a session-start hook never blocks a session
	}
	fmt.Fprintln(cmd.OutOrStdout(), session.Line(id))
	return nil
}

func init() {
	sessionStartCmd.Flags().String("server-url", "", "flmnt server URL (default: login config / FLMNT_SERVER_URL)")
	sessionStartCmd.Flags().String("project", "", "workspace name or id — overrides this repo's own setting (`flmnt setup --project`), which is used before the active workspace")
	rootCmd.AddCommand(sessionStartCmd)
}
