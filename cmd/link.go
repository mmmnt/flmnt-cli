package cmd

import (
	"fmt"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/sync"
	"github.com/spf13/cobra"
)

// linkResolveRemote is the hosted endpoint and login a link is written under; a seam for tests.
var linkResolveRemote = resolveRemoteEndpoint

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Link this machine's local identity to the hosted account you are logged into",
	Long: "Records you wrote under your local identity keep that author forever; the hosted side resolves it\n" +
		"to you through the join table whenever it has to act on it (QRM-ADR-016 §5d). Every `flmnt sync push`\n" +
		"links your identity as it goes. `flmnt link` writes the same rows and pushes nothing — for when you\n" +
		"write to a teammate's local flmnt through a tunnel and someone else pushes the workspace. Needs a\n" +
		"local login (`flmnt login --local`) and a hosted login (`flmnt login`). One hosted account per\n" +
		"identity, ever: a sub another account already holds is refused.",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := identityLoad()
		if err != nil {
			return fmt.Errorf("link refused: this machine has no local identity (run `flmnt login --local`): %w", err)
		}
		remote, hosted, err := linkResolveRemote(cmd)
		if err != nil {
			return fmt.Errorf("link refused: %w", err)
		}
		report, err := sync.Link(remote, append([]string{id.Sub}, id.Formerly()...))
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		who := fmt.Sprintf("%s (%s)", identityOf(hosted), hosted.Sub)
		if len(report.Linked) > 0 {
			fmt.Fprintf(out, "Linked %s to %s.\n", strings.Join(report.Linked, ", "), who)
		}
		if len(report.Held) > 0 {
			fmt.Fprintf(out, "Already linked to %s: %s.\n", who, strings.Join(report.Held, ", "))
		}
		return nil
	},
}

func init() {
	linkCmd.Flags().String("remote-url", "", "hosted flmnt server URL (defaults to the login's)")
	linkCmd.Flags().String("remote-workspace", "", "hosted workspace the request is made under (defaults to the active workspace)")
	rootCmd.AddCommand(linkCmd)
}
