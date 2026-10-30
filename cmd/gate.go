package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/gate"
	"github.com/spf13/cobra"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Context-recency check for the UserPromptSubmit hook",
	Long: "For an agent's prompt hook: reads the latest keyframe of the session's {project}::{session}::keyframe\n" +
		"stream (the session id comes from the hook JSON on stdin) and prints a reminder when there is none or it\n" +
		"is older than --threshold. When no flmnt answers it says FLMNT_NOT_CONNECTED on stderr and exits 0.",
	RunE: runGate,
}

func runGate(cmd *cobra.Command, args []string) error {
	threshold, _ := cmd.Flags().GetDuration("threshold")
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return nil // not configured — stay quiet
	}
	cwd, _ := os.Getwd()
	project := resolveProject(cmd, cwd)
	if project == "" {
		project = envOr("QUORUM_PROJECT_ID", "")
	}
	if project == "" {
		return nil
	}
	session := hookSessionID(cmd.InOrStdin())
	if session == "" {
		return nil // no session named on stdin, so no session keyframe stream to read
	}
	gql, err := graphQLClientFor(cmd, serverURL)
	if err != nil {
		return nil
	}
	out, err := gate.Run(gate.Config{GQL: gql, ProjectID: project, SessionID: session, Threshold: threshold})
	if err != nil {
		reportNotConnected(cmd.ErrOrStderr(), err)
		return nil // a prompt hook never blocks a prompt
	}
	if out != "" {
		fmt.Fprint(cmd.OutOrStdout(), out)
	}
	return nil
}

// hookSessionID reads the agent hook's JSON from stdin and returns its session_id. A terminal on stdin
// (run by hand) is not read, so the command never waits for input nobody will type.
func hookSessionID(in io.Reader) string {
	if f, ok := in.(*os.File); ok {
		if st, err := f.Stat(); err != nil || st.Mode()&os.ModeCharDevice != 0 {
			return ""
		}
	}
	var payload struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(in).Decode(&payload); err != nil {
		return ""
	}
	return payload.SessionID
}

func init() {
	gateCmd.Flags().String("server-url", "", "flmnt server URL (default: login config / QUORUM_SERVER_URL)")
	gateCmd.Flags().Duration("threshold", 1800*time.Second, "Keyframe age threshold (default 30m)")
	gateCmd.Flags().String("project", "", "workspace name or id — overrides this repo's own setting (`flmnt setup --project`), which is used before the active workspace")
	rootCmd.AddCommand(gateCmd)
}
