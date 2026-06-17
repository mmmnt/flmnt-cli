package cmd

import (
	"fmt"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/gate"
	"github.com/spf13/cobra"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Keyframe recency check for UserPromptSubmit hook",
	Long:  "Outputs injection text if the domain stream keyframe is stale or missing. Silent if Core is unreachable.",
	RunE: func(cmd *cobra.Command, args []string) error {
		threshold, _ := cmd.Flags().GetDuration("threshold")
		out, err := gate.Run(gate.Config{
			CoreURL:   envOr("CORE_URL", "http://localhost:3000"),
			ProjectID: envOr("QUORUM_PROJECT_ID", "quorum"),
			Threshold: threshold,
		})
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
