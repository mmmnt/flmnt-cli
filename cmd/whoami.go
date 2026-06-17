package cmd

import (
	"errors"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active identity and workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString("server-url")
		if serverURL == "" {
			serverURL = envOr("QUORUM_SERVER_URL", "")
		}
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
		if cfg.ActiveWorkspaceName != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  (active workspace: %s)\n", identity, cfg.ActiveWorkspaceName)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  (no active workspace)\n", identity)
		}
		return nil
	},
}

func init() {
	whoamiCmd.Flags().String("server-url", "", "flmnt server URL")
	rootCmd.AddCommand(whoamiCmd)
}
