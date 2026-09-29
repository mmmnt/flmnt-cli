package cmd

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/browser"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the flmnt dashboard in a browser",
	Run: func(cmd *cobra.Command, args []string) {
		// The same resolution every other command uses — standing in a configured repo, or signed in,
		// is enough. Hardcoding the local stack here opened a dead URL for every real user.
		url := browser.DashboardURL(resolveAuthServerURL(cmd))
		fmt.Fprintf(cmd.OutOrStdout(), "Opening %s\n", url)
		if err := browser.Open(url); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", err)
		}
	},
}

func init() {
	dashboardCmd.Flags().String("server-url", "", "flmnt server URL (default: login config / QUORUM_SERVER_URL)")
	rootCmd.AddCommand(dashboardCmd)
}
