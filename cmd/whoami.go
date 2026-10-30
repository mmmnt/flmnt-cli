package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/env"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active identity and workspace",
	Long: "Shows who is signed in and which workspace is in force here. The CLI holds one login at a\n" +
		"time and answers by its kind: after `flmnt login --local` the local identity, after `flmnt login`\n" +
		"the stored OAuth login — never by which credential happens to exist, so the local identity a\n" +
		"hosted login keeps is not what answers. Without a login, a loopback server (--server-url\n" +
		"or FLMNT_SERVER_URL) reads as local and anything else as hosted.\n" +
		"--json prints one object {sub,workspace,client_id?} for a headers-helper that\n" +
		"mints a local credential as that person; --client <tool> adds the client id derived for that\n" +
		"tool. Not logged in: nothing on stdout, the reason on stderr, exit 1.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// The same resolution every other command uses, so standing in a configured repo is enough:
		// whoami used to refuse with the server_url its own .flmnt.json was declaring.
		serverURL := resolveAuthServerURL(cmd)
		cfg, _ := authHeaderLoadConfig()
		kind := loginKind(cfg, serverURL)
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			return runWhoamiJSON(cmd, cfg, kind, serverURL)
		}

		var who string
		if kind == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "Not logged in (run `flmnt login` or `flmnt login --local`).")
			return nil
		} else if kind == auth.LoginLocal {
			id, err := identityLoad()
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in locally (run `flmnt login --local`).")
				return nil
			}
			who = "local identity " + id.Sub
			if formerly := id.Formerly(); len(formerly) > 0 {
				who += " (formerly " + joinComma(formerly) + ")"
			}
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

		pinned, pinnedName := "", ""
		if pc, perr := setup.LoadProjectConfig(""); perr == nil {
			pinned, pinnedName = pc.ProjectID, pc.ProjectName
		}
		fmt.Fprintln(cmd.OutOrStdout(), whoamiLine(who, cfg.ActiveWorkspaceName, pinned, pinnedName))
		if setup.UsesLegacyProjectConfig(".") {
			fmt.Fprintf(cmd.ErrOrStderr(), "this checkout's %s is the former name: rename it %s\n", setup.LegacyProjectConfigFile, setup.ProjectConfigFile)
		}
		return nil
	},
}

// loginKind is which login answers: the recorded session's, or — with no session — local for a
// loopback server and hosted for any other; "" when there is neither a session nor a server.
func loginKind(cfg auth.CLIConfig, serverURL string) string {
	if kind := cfg.LoginKind(); kind != "" {
		return kind
	}
	switch {
	case serverURL == "":
		return ""
	case httpx.IsLoopback(serverURL):
		return auth.LoginLocal
	default:
		return auth.LoginHosted
	}
}

// runWhoamiJSON is what a local headers-helper asks (`flmnt whoami --json --client <tool>`) to learn
// who to mint for. A local login answers with the derived sub and the client id derived for the
// tool; a hosted login with the stored token's sub and its registered OAuth client. Nothing on
// stdout when there is no login: the helper then blocks.
func runWhoamiJSON(cmd *cobra.Command, cfg auth.CLIConfig, kind, serverURL string) error {
	tool, _ := cmd.Flags().GetString("client")
	out := map[string]string{}
	if kind == "" {
		return fmt.Errorf("whoami: not logged in (run `flmnt login` or `flmnt login --local`)")
	}
	if kind == auth.LoginLocal {
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
			out["client_id"] = firstNonEmpty(env.Get("CLIENT_ID"), cfg.ClientID)
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
