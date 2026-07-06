package cmd

import (
	"fmt"
	"os"

	"github.com/mmmnt/flmnt-cli/internal/brief"
	"github.com/spf13/cobra"
)

var briefCmd = &cobra.Command{
	Use:   "brief",
	Short: "SessionStart briefing — inject the project's current reasoning state from flmnt",
	Long: "Reads the latest keyframe + recent decisions + recent mistakes for the active project and\n" +
		"prints a compact briefing for a SessionStart hook to inject — the read half of the continuity\n" +
		"loop. Read-only; fails quiet (prints nothing) when there's no memory or no config.",
	RunE: runBrief,
}

func runBrief(cmd *cobra.Command, args []string) error {
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return nil // not configured — stay quiet
	}
	cwd, _ := os.Getwd()
	project := resolveProject(cmd, cwd)
	if project == "" {
		return nil
	}
	gql, err := graphQLClientFor(cmd, serverURL)
	if err != nil {
		return nil // not configured — stay quiet
	}
	text, err := brief.Render(brief.Config{GQL: gql, ProjectID: project})
	if err != nil || text == "" {
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), text)
	return nil
}

func init() {
	briefCmd.Flags().String("server-url", "", "flmnt server URL (default: login config / QUORUM_SERVER_URL; localhost for a local stack)")
	briefCmd.Flags().String("project", "", "flmnt project id (default: active workspace)")
	rootCmd.AddCommand(briefCmd)
}
