package cmd

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/localadmin"
	"github.com/spf13/cobra"
)

var (
	loginRunDeviceFlow   = auth.RunDeviceFlow
	loginRunPKCEFlow     = auth.RunPKCEFlow
	loginStoreToken      = auth.StoreToken
	loginDeleteAllTokens = auth.DeleteAllTokens
	loginDiscover        = discoverOAuth
	loginLoadConfig      = auth.LoadConfig
	loginSaveConfig      = auth.SaveConfig
	loginNow             = time.Now
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with flmnt via OAuth2 (browser PKCE, or --device for headless); --local declares the identity a local flmnt mints for",
	RunE: func(cmd *cobra.Command, args []string) error {
		if local, _ := cmd.Flags().GetBool("local"); local {
			return runLocalLogin(cmd)
		}
		serverURL, _ := cmd.Flags().GetString("server-url")
		if serverURL == "" {
			serverURL = envOr("QUORUM_SERVER_URL", "")
		}
		if serverURL == "" {
			return fmt.Errorf("--server-url or QUORUM_SERVER_URL is required")
		}

		device, _ := cmd.Flags().GetBool("device")
		flagAuth, _ := cmd.Flags().GetString("auth-url")
		flagToken, _ := cmd.Flags().GetString("token-url")
		flagClient, _ := cmd.Flags().GetString("client-id")
		flagDevice, _ := cmd.Flags().GetString("device-url")
		envClient := envOr("QUORUM_CLIENT_ID", "")

		cfg, _ := loginLoadConfig()
		// Always re-discover: the server's advertised authorization server is authoritative, so a moved
		// broker (new DNS) is picked up even when config.json still caches the old endpoints. Discovery
		// outranks the cache in resolveLoginEndpoints; flags/env still win, and a failed discovery falls
		// back to the cache. Without this a stale cache silently re-auths against the retired broker,
		// minting tokens the resource server rejects ("login doesn't stick").
		doc, _ := loginDiscover(serverURL)
		authURL, tokenURL, clientID := resolveLoginEndpoints(flagAuth, flagToken, flagClient, envClient, cfg, doc)
		deviceURL := firstNonEmpty(flagDevice, envOr("QUORUM_DEVICE_URL", ""), doc.DeviceAuthorizationEndpoint)
		revocationEndpoint := firstNonEmpty(doc.RevocationEndpoint, cfg.RevocationEndpoint)

		var tokens auth.TokenSet
		if device {
			if deviceURL == "" || tokenURL == "" || clientID == "" {
				return fmt.Errorf("--device-url, --token-url and --client-id are required with --device (and could not be discovered from %s)", serverURL)
			}
			t, err := loginRunDeviceFlow(auth.DeviceConfig{
				DeviceURL: deviceURL,
				TokenURL:  tokenURL,
				ClientID:  clientID,
				Scope:     "openid email profile",
			}, func(d auth.DeviceAuthResponse) error {
				fmt.Fprintf(cmd.OutOrStdout(), "To sign in, visit %s and enter code: %s\n", d.VerificationURI, d.UserCode)
				return nil
			}, nil, nil)
			if err != nil {
				return fmt.Errorf("device login failed: %w", err)
			}
			tokens = t
		} else {
			if authURL == "" || tokenURL == "" || clientID == "" {
				return fmt.Errorf("could not resolve OAuth endpoints from %s; pass --client-id (auth-url/token-url are discovered)", serverURL)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Opening browser for authentication...")
			t, err := loginRunPKCEFlow(auth.PKCEConfig{
				AuthURL:     authURL,
				TokenURL:    tokenURL,
				ClientID:    clientID,
				RedirectURI: "http://127.0.0.1:9877/",
			}, openInBrowser)
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}
			tokens = t
		}

		// One login at a time: this login replaces whatever session was here — every token stored for
		// any server — before its own token is stored. The local identity is not a session and stays:
		// a person signed in to the cloud still pushes and links their local records as it.
		if _, err := loginDeleteAllTokens(); err != nil {
			return fmt.Errorf("clearing the previous login: %w", err)
		}
		if err := loginStoreToken(serverURL, tokens); err != nil {
			return fmt.Errorf("storing token: %w", err)
		}
		claims, _ := auth.DecodeUnverified(firstNonEmpty(tokens.IDToken, tokens.AccessToken))
		_ = loginSaveConfig(hostedSession(cfg, serverURL, claims, auth.CLIConfig{
			AuthURL:            authURL,
			TokenURL:           tokenURL,
			ClientID:           clientID,
			RevocationEndpoint: revocationEndpoint,
		}, loginNow()))
		fmt.Fprintln(cmd.OutOrStdout(), "Login successful. Token stored in "+auth.StorageDescription()+".")
		fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s; every command now talks to %s until `flmnt logout`.\n", identityOf(claims), serverURL)
		return nil
	},
}

// hostedSession is the config a hosted login records: the kind, the server, who the token says we
// are, and the OAuth endpoints logout and refresh need. The active workspace carries over only when
// the previous session was a hosted login to this same server — a workspace names nothing on any
// other server, and nothing on the local harness.
func hostedSession(prev auth.CLIConfig, serverURL string, claims auth.Claims, endpoints auth.CLIConfig, now time.Time) auth.CLIConfig {
	cfg := auth.CLIConfig{
		Login:              auth.LoginHosted,
		ServerURL:          serverURL,
		Sub:                claims.Sub,
		Identity:           identityOf(claims),
		LoggedInAt:         now.UTC().Format(time.RFC3339),
		AuthURL:            endpoints.AuthURL,
		TokenURL:           endpoints.TokenURL,
		ClientID:           endpoints.ClientID,
		RevocationEndpoint: endpoints.RevocationEndpoint,
	}
	if prev.LoginKind() == auth.LoginHosted && sameOrigin(prev.ServerURL, serverURL) {
		cfg.ActiveWorkspaceID, cfg.ActiveWorkspaceName = prev.ActiveWorkspaceID, prev.ActiveWorkspaceName
	}
	return cfg
}

func sameOrigin(a, b string) bool {
	ua, erra := url.Parse(a)
	ub, errb := url.Parse(b)
	if erra != nil || errb != nil {
		return a == b
	}
	return ua.Scheme == ub.Scheme && ua.Host == ub.Host
}

func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func init() {
	loginCmd.Flags().String("server-url", "", "flmnt MCP server URL")
	loginCmd.Flags().String("auth-url", "", "OAuth2 authorization endpoint (default: from discovery)")
	loginCmd.Flags().String("token-url", "", "OAuth2 token endpoint (default: from discovery)")
	loginCmd.Flags().String("client-id", "", "OAuth2 client ID")
	loginCmd.Flags().Bool("device", false, "Use the device-authorization grant (headless, no browser)")
	loginCmd.Flags().String("device-url", "", "Device-authorization endpoint (with --device)")
	loginCmd.Flags().Bool("local", false, "record the local login (derived from this machine and the OS user) that a local flmnt mints credentials for")
	loginCmd.Flags().String("admin-url", localadmin.DefaultBaseURL, "with --local: the local broker's admin listener the CLI approves its own device code through")
	rootCmd.AddCommand(loginCmd)
}
