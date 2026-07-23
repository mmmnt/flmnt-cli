package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

var (
	loginRunDeviceFlow = auth.RunDeviceFlow
	loginRunPKCEFlow   = auth.RunPKCEFlow
	loginStoreToken    = auth.StoreToken
	loginDiscover      = discoverOAuth
	loginLoadConfig    = auth.LoadConfig
	loginSaveConfig    = auth.SaveConfig
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with flmnt via OAuth2 (browser PKCE, or --device for headless)",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		if err := loginStoreToken(serverURL, tokens); err != nil {
			return fmt.Errorf("storing token: %w", err)
		}
		cfg.ServerURL = serverURL
		cfg.AuthURL = authURL
		cfg.TokenURL = tokenURL
		cfg.ClientID = clientID
		cfg.RevocationEndpoint = revocationEndpoint
		_ = loginSaveConfig(cfg)
		fmt.Fprintln(cmd.OutOrStdout(), "Login successful. Token stored in "+auth.StorageDescription()+".")
		return nil
	},
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
	rootCmd.AddCommand(loginCmd)
}
