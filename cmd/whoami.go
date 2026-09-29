package cmd

import (
	"errors"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active identity and workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		// The same resolution every other command uses, so standing in a configured repo is enough:
		// whoami used to refuse with the server_url its own .quorum.json was declaring.
		serverURL := resolveAuthServerURL(cmd)
		if serverURL == "" {
			return fmt.Errorf("--server-url or QUORUM_SERVER_URL is required")
		}

		tokens, err := auth.LoadToken(serverURL)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in.")
				return nil
			}
			return err
		}

		idToken := tokens.IDToken
		if idToken == "" {
			idToken = tokens.AccessToken
		}
		claims, err := auth.DecodeUnverified(idToken)
		if err != nil {
			return fmt.Errorf("decoding token: %w", err)
		}

		cfg, _ := auth.LoadConfig()
		identity := claims.Email
		if identity == "" {
			identity = claims.Username
		}
		if identity == "" {
			identity = claims.Sub
		}
		pinned, pinnedName := "", ""
		if pc, perr := setup.LoadProjectConfig(""); perr == nil {
			pinned, pinnedName = pc.ProjectID, pc.ProjectName
		}
		fmt.Fprintln(cmd.OutOrStdout(), whoamiLine(identity, cfg.ActiveWorkspaceName, pinned, pinnedName))
		return nil
	},
}

func init() {
	whoamiCmd.Flags().String("server-url", "", "flmnt server URL")
	rootCmd.AddCommand(whoamiCmd)
}

// whoamiLine says which workspace is actually in force. A repo that has been set up records into its
// OWN workspace regardless of the active one, so naming the active workspace there would answer a
// question nobody asked and hide the one that governs. The pinned workspace is shown by NAME when the
// repo config records one; a repo set up before that field existed still shows the id, which is the
// only truth available there.
func whoamiLine(identity, activeName, pinned, pinnedName string) string {
	if pinnedName != "" {
		return fmt.Sprintf("%s  (this repo records into: %s)", identity, pinnedName)
	}
	if pinned != "" {
		return fmt.Sprintf("%s  (this repo records into: %s)", identity, pinned)
	}
	if activeName != "" {
		return fmt.Sprintf("%s  (active workspace: %s)", identity, activeName)
	}
	return fmt.Sprintf("%s  (no active workspace)", identity)
}
