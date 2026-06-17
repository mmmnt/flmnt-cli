package cmd

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/browser"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the Quorum dashboard in a browser",
	Run: func(cmd *cobra.Command, args []string) {
		url := browser.ResolveURL()
		fmt.Fprintf(cmd.OutOrStdout(), "Opening %s\n", url)
		if err := browser.Open(url); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
