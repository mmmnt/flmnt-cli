package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "flmnt",
	Short: "flmnt developer CLI",
	Long:  "flmnt is the developer CLI for the flmnt platform. Manages authentication, workspaces, and developer tooling.",
	// A runtime failure — no corpus in that workspace, no local broker, a missing argument — has
	// nothing to do with the flags, and printing them buries the reason above a screen of help. Set on
	// the root so every command inherits rather than each one remembering.
	//
	// It silences the FLAG-parse errors too, which is not what anyone wants: an unknown flag is the one
	// case where the flag list is the answer. Cobra checks the ROOT's SilenceUsage, so a child cannot
	// opt back in — the flag-error path has to print usage itself, which SetFlagErrorFunc does below.
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// The exception to SilenceUsage: a flag-parse error IS about the flags. Printed here because the
	// root's silence cannot be overridden per command; the error itself is still reported by cobra
	// afterwards, so the reader gets the flag list and then the reason.
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		c.Println(c.UsageString())
		return err
	})
}
