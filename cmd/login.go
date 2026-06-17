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
	loginStoreToken    = auth.StoreToken
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Quorum via OAuth2 (browser PKCE, or --device for headless)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString("server-url")
		if serverURL == "" {
			serverURL = envOr("QUORUM_SERVER_URL", "")
		}
		if serverURL == "" {
			return fmt.Errorf("--server-url or QUORUM_SERVER_URL is required")
		}

		tokenURL, _ := cmd.Flags().GetString("token-url")
		clientID, _ := cmd.Flags().GetString("client-id")

		if device, _ := cmd.Flags().GetBool("device"); device {
			deviceURL, _ := cmd.Flags().GetString("device-url")
			if deviceURL == "" || tokenURL == "" || clientID == "" {
				return fmt.Errorf("--device-url, --token-url and --client-id are required with --device")
			}
			tokens, err := loginRunDeviceFlow(auth.DeviceConfig{
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
			if err := loginStoreToken(serverURL, tokens); err != nil {
				return fmt.Errorf("storing token: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Login successful. Token stored in OS keychain.")
			return nil
		}

		authURL, _ := cmd.Flags().GetString("auth-url")
		if authURL == "" || tokenURL == "" || clientID == "" {
			return fmt.Errorf("--auth-url, --token-url and --client-id are required")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Opening browser for authentication...")
		tokens, err := auth.RunPKCEFlow(auth.PKCEConfig{
			AuthURL:     authURL,
			TokenURL:    tokenURL,
			ClientID:    clientID,
			RedirectURI: "http://127.0.0.1:9877",
		}, openInBrowser)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		if err := loginStoreToken(serverURL, tokens); err != nil {
			return fmt.Errorf("storing token: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Login successful. Token stored in OS keychain.")
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
	loginCmd.Flags().String("server-url", "", "Quorum server URL")
	loginCmd.Flags().String("auth-url", "", "OAuth2 authorization endpoint")
	loginCmd.Flags().String("token-url", "", "OAuth2 token endpoint")
	loginCmd.Flags().String("client-id", "", "OAuth2 client ID")
	loginCmd.Flags().Bool("device", false, "Use the device-authorization grant (headless, no browser)")
	loginCmd.Flags().String("device-url", "", "Device-authorization endpoint (with --device)")
	rootCmd.AddCommand(loginCmd)
}
