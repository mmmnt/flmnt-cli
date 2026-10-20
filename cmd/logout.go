package cmd

import (
	"errors"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

var (
	logoutLoadConfig      = auth.LoadConfig
	logoutClearConfig     = auth.ClearConfig
	logoutLoadToken       = auth.LoadToken
	logoutDeleteAllTokens = auth.DeleteAllTokens
	logoutRevoke          = auth.RevokeRefreshToken
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out: clear the session and every stored token; the local identity stays",
	Long: "Ends the login: the session config, every OAuth token the keychain holds for any server, and\n" +
		"the encrypted file keyring of older builds are all removed, and the hosted refresh token is revoked\n" +
		"when the login recorded a revocation endpoint. The local identity (`flmnt login --local`'s derived\n" +
		"sub) is who this machine is, not a login, and stays: the next `flmnt login --local` reuses it.\n" +
		"--local ends only a local session.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if local, _ := cmd.Flags().GetBool("local"); local {
			return runLocalLogout(cmd)
		}
		cfg, _ := logoutLoadConfig()
		serverURL := firstNonEmpty(flagString(cmd, "server-url"), envOr("QUORUM_SERVER_URL", ""), cfg.ServerURL)
		revokeURL := firstNonEmpty(flagString(cmd, "revoke-url"), cfg.RevocationEndpoint)
		clientID := firstNonEmpty(flagString(cmd, "client-id"), cfg.ClientID)

		// Revoke server-side first, while the refresh token is still here to revoke.
		var refreshToken string
		if serverURL != "" {
			tokens, err := logoutLoadToken(serverURL)
			if err == nil {
				refreshToken = tokens.RefreshToken
			} else if !errors.Is(err, auth.ErrNotFound) {
				return fmt.Errorf("loading stored token: %w", err)
			}
		}
		if revokeURL != "" && refreshToken != "" && clientID != "" {
			if err := logoutRevoke(revokeURL, clientID, refreshToken); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: refresh-token revocation failed: %v\n", err)
			}
		}

		// Then every token, whatever the config named or failed to name: a logout that cleared the
		// config but left the token behind is how whoami kept answering with an old account. The local
		// identity is not a credential and is not touched.
		removed, err := logoutDeleteAllTokens()
		if err != nil {
			return fmt.Errorf("clearing keychain: %w", err)
		}
		if err := logoutClearConfig(); err != nil {
			return fmt.Errorf("clearing config: %w", err)
		}

		identitySub := ""
		if id, err := identityLoad(); err == nil {
			identitySub = id.Sub
		}
		fmt.Fprintln(cmd.OutOrStdout(), logoutLine(cfg, auth.OriginsOf(removed), identitySub))
		return nil
	},
}

// logoutLine says what was ended, what was cleared and what stays, so a person can see exactly what
// is left: the local identity when the machine has one, nothing otherwise.
func logoutLine(cfg auth.CLIConfig, origins []string, identitySub string) string {
	line := "Logged out."
	switch cfg.LoginKind() {
	case auth.LoginLocal:
		line += " Ended the local login " + cfg.Identity + "."
	case auth.LoginHosted:
		line += " Ended the hosted login " + firstNonEmpty(cfg.Identity, "to "+cfg.ServerURL) + "."
	}
	if len(origins) > 0 {
		line += " Cleared tokens for " + joinComma(origins) + "."
	}
	if identitySub != "" {
		return line + " The local identity " + identitySub + " stays; `flmnt login --local` reuses it."
	}
	return line + " Nothing remains."
}

func init() {
	logoutCmd.Flags().String("server-url", "", "flmnt server URL whose refresh token to revoke (default: the login's)")
	logoutCmd.Flags().String("revoke-url", "", "OAuth2 token-revocation endpoint")
	logoutCmd.Flags().String("client-id", "", "OAuth2 client ID")
	logoutCmd.Flags().Bool("local", false, "remove only the local login")
	rootCmd.AddCommand(logoutCmd)
}
