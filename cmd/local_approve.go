package cmd

import (
	"bufio"
	"fmt"
	"strings"

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
		"for the client's name. A code a teammate claimed from their machine (`flmnt local claim`) is\n" +
		"approved for THEM, and the prompt says so. Reaching the admin listener is the proof that this is\n" +
		"the machine.",
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
		// The tool whose client id is derived: a web client is named by the broker (its registered name,
		// else its client id); a device code is the CLI's own sign-in, so the CLI's tool name.
		tool, what, done := cliTool, "this machine's flmnt CLI device sign-in", "Approved."
		if pending.Kind == "authorization" {
			tool = firstNonEmpty(pending.ClientName, pending.ClientID)
			what = fmt.Sprintf("%s (%s) for sub %s", tool, pending.RedirectURI, id.Sub)
			if pending.ClaimedSub != "" {
				what = fmt.Sprintf("%s (%s) for sub %s (claimed from another machine; this machine's identity is not used)", tool, pending.RedirectURI, pending.ClaimedSub)
			}
			done = "Approved. Return to your browser."
		}
		if yes, _ := cmd.Flags().GetBool("yes"); yes {
			fmt.Fprintf(cmd.OutOrStdout(), "Approving %s.\n", what)
		} else if !confirm(cmd, "Approve "+what+"?") {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing approved.")
			return nil
		}
		if err := localAdminApprove(adminURL, code, id.Sub, id.ClientID(tool)); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), done)
		return nil
	},
}

// confirm asks the question on stdout and reads one line from stdin; only y/yes is a yes. A closed
// stdin is a no — nothing is approved by default.
func confirm(cmd *cobra.Command, question string) bool {
	fmt.Fprint(cmd.OutOrStdout(), question+" [y/N] ")
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	fmt.Fprintln(cmd.OutOrStdout())
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

func init() {
	localApproveCmd.Flags().String("admin-url", localadmin.DefaultBaseURL, "the local broker's admin listener (loopback only)")
	localApproveCmd.Flags().BoolP("yes", "y", false, "approve without asking for confirmation")
	localCmd.AddCommand(localApproveCmd)
}
