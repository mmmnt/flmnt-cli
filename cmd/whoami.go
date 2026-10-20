package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active identity and workspace",
	Long: "Shows who is signed in and which workspace is in force here. --json prints the stored\n" +
		"login's claims {sub,email,username,workspace} decoded offline from the keychain, for a\n" +
		"headers-helper that mints a local credential as that person; not logged in → exit 1.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			return runWhoamiJSON(cmd)
		}
		// The same resolution every other command uses, so standing in a configured repo is enough:
		// whoami used to refuse with the server_url its own .quorum.json was declaring.
		serverURL := resolveAuthServerURL(cmd)
		if serverURL == "" {
			return fmt.Errorf("--server-url or QUORUM_SERVER_URL is required")
		}

		tokens, err := authHeaderLoadToken(serverURL)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in.")
				return nil
			}
			return err
		}
		claims, err := auth.DecodeUnverified(firstNonEmpty(tokens.IDToken, tokens.AccessToken))
		if err != nil {
			return fmt.Errorf("decoding token: %w", err)
		}

		cfg, _ := authHeaderLoadConfig()
		pinned, pinnedName := "", ""
		if pc, perr := setup.LoadProjectConfig(""); perr == nil {
			pinned, pinnedName = pc.ProjectID, pc.ProjectName
		}
		fmt.Fprintln(cmd.OutOrStdout(), whoamiLine(identityOf(claims), cfg.ActiveWorkspaceName, pinned, pinnedName))
		return nil
	},
}

// runWhoamiJSON answers "who is logged in" for a machine: the stored login's claims, decoded offline.
// It reads the login `flmnt login` stored rather than the server the session resolves to, because the
// local hook wrapper points the session at the harness — which holds no keychain token — while the
// person the helper mints for is the one logged in to flmnt.
func runWhoamiJSON(cmd *cobra.Command) error {
	serverURL := loginServerURL(cmd)
	tokens, err := authHeaderLoadToken(serverURL)
	if err != nil {
		return fmt.Errorf("whoami: not logged in to %s (run `flmnt login`)", serverURL)
	}
	claims, err := auth.DecodeUnverified(firstNonEmpty(tokens.IDToken, tokens.AccessToken))
	if err != nil {
		return fmt.Errorf("whoami: decoding the stored token: %w", err)
	}
	cfg, _ := authHeaderLoadConfig()
	workspace := cfg.ActiveWorkspaceID
	if pc, perr := setup.LoadProjectConfig(""); perr == nil && pc.ProjectID != "" {
		workspace = pc.ProjectID
	}
	out, err := json.Marshal(map[string]string{
		"sub":       claims.Sub,
		"email":     claims.Email,
		"username":  claims.Username,
		"workspace": workspace,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

// loginServerURL is the server the stored login belongs to: an explicit --server-url, else the one
// `flmnt login` wrote to the config, else the session's resolution chain.
func loginServerURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("server-url"); v != "" {
		return v
	}
	if cfg, err := authHeaderLoadConfig(); err == nil && cfg.ServerURL != "" {
		return cfg.ServerURL
	}
	return resolveAuthServerURL(cmd)
}

func identityOf(claims auth.Claims) string {
	return firstNonEmpty(claims.Email, claims.Username, claims.Sub)
}

func init() {
	whoamiCmd.Flags().String("server-url", "", "flmnt server URL")
	whoamiCmd.Flags().Bool("json", false, "print the stored login's claims as one JSON object {sub,email,username,workspace}; exit 1 when not logged in")
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
