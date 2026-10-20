package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active identity and workspace",
	Long: "Shows who is signed in and which workspace is in force here. On a local server (loopback)\n" +
		"the identity is the local login (`flmnt login --local`); elsewhere it is the stored OAuth login.\n" +
		"--json prints one object {sub,workspace,client_id?} for a headers-helper that\n" +
		"mints a local credential as that person; --client <tool> adds the client id derived for that\n" +
		"tool. Not logged in: nothing on stdout, the reason on stderr, exit 1.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// The same resolution every other command uses, so standing in a configured repo is enough:
		// whoami used to refuse with the server_url its own .quorum.json was declaring.
		serverURL := resolveAuthServerURL(cmd)
		if serverURL == "" {
			return fmt.Errorf("--server-url or QUORUM_SERVER_URL is required")
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			return runWhoamiJSON(cmd, serverURL)
		}

		var who string
		if httpx.IsLoopback(serverURL) {
			id, err := identityLoad()
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in locally (run `flmnt login --local`).")
				return nil
			}
			who = "local identity " + id.Sub
		} else {
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
			who = identityOf(claims)
		}

		cfg, _ := authHeaderLoadConfig()
		pinned, pinnedName := "", ""
		if pc, perr := setup.LoadProjectConfig(""); perr == nil {
			pinned, pinnedName = pc.ProjectID, pc.ProjectName
		}
		fmt.Fprintln(cmd.OutOrStdout(), whoamiLine(who, cfg.ActiveWorkspaceName, pinned, pinnedName))
		return nil
	},
}

// runWhoamiJSON is what a local headers-helper asks (`flmnt whoami --json --client <tool>`) to learn
// who to mint for. A loopback server answers from the local login — the derived sub and the client
// id derived for the tool; a hosted server from the stored login and its registered OAuth client.
// Nothing on stdout when there is no login of the kind asked for: the helper then blocks.
func runWhoamiJSON(cmd *cobra.Command, serverURL string) error {
	tool, _ := cmd.Flags().GetString("client")
	cfg, _ := authHeaderLoadConfig()
	out := map[string]string{}
	if httpx.IsLoopback(serverURL) {
		id, err := identityLoad()
		if err != nil {
			return fmt.Errorf("whoami: %w", err)
		}
		out["sub"] = id.Sub
		if tool != "" {
			out["client_id"] = id.ClientID(tool)
		}
	} else {
		tokens, err := authHeaderLoadToken(serverURL)
		if err != nil {
			return fmt.Errorf("whoami: not logged in to %s (run `flmnt login`)", serverURL)
		}
		claims, err := auth.DecodeUnverified(firstNonEmpty(tokens.IDToken, tokens.AccessToken))
		if err != nil {
			return fmt.Errorf("whoami: decoding the stored token: %w", err)
		}
		out["sub"] = claims.Sub
		if tool != "" {
			out["client_id"] = firstNonEmpty(envOr("QUORUM_CLIENT_ID", ""), cfg.ClientID)
		}
	}
	out["workspace"] = cfg.ActiveWorkspaceID
	if pc, perr := setup.LoadProjectConfig(""); perr == nil && pc.ProjectID != "" {
		out["workspace"] = pc.ProjectID
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	return nil
}

func identityOf(claims auth.Claims) string {
	return firstNonEmpty(claims.Email, claims.Username, claims.Sub)
}

func init() {
	whoamiCmd.Flags().String("server-url", "", "flmnt server URL")
	whoamiCmd.Flags().Bool("json", false, "print the identity as one JSON object {sub,workspace,client_id?}; exit 1 when not logged in")
	whoamiCmd.Flags().String("client", "", "with --json: the tool whose client id to include (derived locally; the registered OAuth client when hosted)")
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
