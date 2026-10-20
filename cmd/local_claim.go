package cmd

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/localclaim"
	"github.com/spf13/cobra"
)

var (
	localClaimDescribe = localclaim.Describe
	localClaimClaim    = localclaim.Claim
)

var localClaimCmd = &cobra.Command{
	Use:   "claim <code>",
	Short: "Claim a pending sign-in on someone else's local flmnt for this machine's local identity",
	Long: "A teammate's local flmnt (behind a tunnel) shows your client a code. Before its owner approves\n" +
		"the code, claim it from YOUR machine: the sign-in is then approved for your local identity, not\n" +
		"the host's, and your writes are yours (QRM-ADR-016 §5a). --server-url names their flmnt (the\n" +
		"page's origin); `flmnt login --local` must have run here first, so your identity exists. Then\n" +
		"tell the owner your code; they run `flmnt local approve <code>`.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		code := args[0]
		id, err := identityLoad()
		if err != nil {
			return fmt.Errorf("local claim refused: %w", err)
		}
		serverURL := resolveAuthServerURL(cmd)
		if serverURL == "" {
			return fmt.Errorf("local claim refused: --server-url must name the flmnt whose page showed the code (its origin)")
		}
		pending, err := localClaimDescribe(serverURL, code)
		if err != nil {
			return err
		}
		tool := pending.ClientName
		what := fmt.Sprintf("%s (%s) on %s for your identity %s", tool, pending.RedirectURI, serverURL, id.Sub)
		if yes, _ := cmd.Flags().GetBool("yes"); yes {
			fmt.Fprintf(cmd.OutOrStdout(), "Claiming %s.\n", what)
		} else if !confirm(cmd, "Claim "+what+"?") {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing claimed.")
			return nil
		}
		if err := localClaimClaim(serverURL, code, id.Sub, id.ClientID(tool)); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Claimed. Tell the owner of %s your code %s; they approve it with `flmnt local approve %s`.\n", serverURL, code, code)
		return nil
	},
}

func init() {
	localClaimCmd.Flags().String("server-url", "", "the flmnt whose approval page showed the code (its origin, e.g. the tunnel URL)")
	localClaimCmd.Flags().BoolP("yes", "y", false, "claim without asking for confirmation")
	localCmd.AddCommand(localClaimCmd)
}
