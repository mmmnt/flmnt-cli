package cmd

import "github.com/spf13/cobra"

// localCmd groups the operations on the local flmnt node: running its stack (up, down, status,
// compose) and the sign-ins its broker mints for the local login (approve, claim).
var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Operations on the local flmnt harness",
}

func init() {
	rootCmd.AddCommand(localCmd)
}
