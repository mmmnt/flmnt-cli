package cmd

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/localadmin"
	"github.com/spf13/cobra"
)

var (
	localAdminPending = localadmin.Pending
	localAdminApprove = localadmin.Approve
)

var localApproveCmd = &cobra.Command{
	Use:   "approve <code>",
	Short: "Approve a pending sign-in on the local broker for this machine's local login",
	Long: "A client that signs in to the local flmnt (claude.ai, Claude Code) is shown a code and this\n" +
		"command. It asks the broker's loopback-only admin listener what the code is waiting for, shows it,\n" +
		"and approves it for the local login (`flmnt login --local`): the sub, and the client id derived\n" +
		"for the client's name. Reaching the admin listener is the proof that this is the machine.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		code := args[0]
		adminURL, _ := cmd.Flags().GetString("admin-url")
		id, err := identityLoad()
		if err != nil {
			return fmt.Errorf("local approve refused: %w", err)
		}
		pending, err := localAdminPending(adminURL, code)
		if err != nil {
			return err
		}
		tool := firstNonEmpty(pending.ClientName, pending.ClientID)
		fmt.Fprintf(cmd.OutOrStdout(), "Approve %s (%s) for sub %s? [y/N] ", tool, pending.RedirectURI, id.Sub)
		fmt.Fprintln(cmd.OutOrStdout(), "y")
		if err := localAdminApprove(adminURL, code, id.Sub, id.ClientID(tool)); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Approved. Return to your browser.")
		return nil
	},
}

func init() {
	localApproveCmd.Flags().String("admin-url", localadmin.DefaultBaseURL, "the local broker's admin listener (loopback only)")
	localApproveCmd.Flags().BoolP("yes", "y", false, "approve without asking for confirmation")
	localCmd.AddCommand(localApproveCmd)
}
