package cmd

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/proxy"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the local MCP proxy daemon",
	Long: `Starts a local reverse proxy that reads the Quorum token from the OS keychain
and injects Authorization: Bearer headers on outbound MCP requests.
.mcp.json should point to http://localhost:PROXY_PORT/mcp.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		serverURL := envOr("QUORUM_SERVER_URL", "")
		if serverURL == "" {
			serverURL, _ = cmd.Flags().GetString("server-url")
		}
		if serverURL == "" {
			if pc, err := setup.LoadProjectConfig(""); err == nil {
				serverURL = pc.ServerURL
			}
		}
		if serverURL == "" {
			return fmt.Errorf("--server-url, QUORUM_SERVER_URL, or quorum setup must provide a server URL")
		}

		addr := fmt.Sprintf(":%d", port)
		fmt.Fprintf(cmd.OutOrStdout(), "Quorum proxy listening on %s → %s\n", addr, serverURL)

		return proxy.ListenAndServe(addr, proxy.Config{
			TargetURL: serverURL,
			TokenFetcher: func() (string, error) {
				t, err := auth.LoadToken(serverURL)
				if err != nil {
					return "", nil // unauthenticated state — proxy will return 401
				}
				return t.AccessToken, nil
			},
		})
	},
}

func init() {
	proxyCmd.Flags().Int("port", 9876, "Port to listen on")
	proxyCmd.Flags().String("server-url", "", "Quorum server URL")
	rootCmd.AddCommand(proxyCmd)
}
