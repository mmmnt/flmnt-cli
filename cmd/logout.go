package cmd

import (
	"errors"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out and revoke local credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString("server-url")
		if serverURL == "" {
			serverURL = envOr("QUORUM_SERVER_URL", "")
		}
		revokeURL, _ := cmd.Flags().GetString("revoke-url")
		clientID, _ := cmd.Flags().GetString("client-id")

		var refreshToken string
		if serverURL != "" {
			tokens, err := auth.LoadToken(serverURL)
			if err == nil {
				refreshToken = tokens.RefreshToken
			} else if !errors.Is(err, auth.ErrNotFound) {
				return fmt.Errorf("loading stored token: %w", err)
			}
		}

		if revokeURL != "" && refreshToken != "" && clientID != "" {
			if err := auth.RevokeRefreshToken(revokeURL, clientID, refreshToken); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: refresh-token revocation failed: %v\n", err)
			}
		}

		if serverURL != "" {
			if err := auth.DeleteToken(serverURL); err != nil {
				return fmt.Errorf("clearing keychain: %w", err)
			}
		}
		if err := auth.ClearConfig(); err != nil {
			return fmt.Errorf("clearing config: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
		return nil
	},
}

func init() {
	logoutCmd.Flags().String("server-url", "", "flmnt server URL")
	logoutCmd.Flags().String("revoke-url", "", "OAuth2 token-revocation endpoint")
	logoutCmd.Flags().String("client-id", "", "OAuth2 client ID")
	rootCmd.AddCommand(logoutCmd)
}
