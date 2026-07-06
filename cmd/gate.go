package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/gate"
	"github.com/spf13/cobra"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Context-recency check for the UserPromptSubmit hook",
	Long:  "Outputs reminder text if your saved context is stale or missing. Silent if the service is unreachable.",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		gql, err := graphQLClientFor(cmd, serverURL)
		if err != nil {
			return nil
		}
		out, err := gate.Run(gate.Config{GQL: gql, ProjectID: project, Threshold: threshold})
		if err != nil {
			return err
		}
		if out != "" {
			fmt.Fprint(cmd.OutOrStdout(), out)
		}
		return nil
	},
}

func init() {
	gateCmd.Flags().Duration("threshold", 1800*time.Second, "Keyframe age threshold (default 30m)")
	rootCmd.AddCommand(gateCmd)
}
