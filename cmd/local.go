package cmd

import "github.com/spf13/cobra"

// localCmd groups the operations on the local flmnt harness (the compose stack a checkout runs with
// `pnpm local:start`), which runs the real OAuth broker and mints for the local login.
var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Operations on the local flmnt harness",
}

func init() {
	rootCmd.AddCommand(localCmd)
}
