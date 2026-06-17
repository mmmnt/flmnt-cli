package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var (
	authHeaderLoadToken  = auth.LoadToken
	authHeaderStoreToken = auth.StoreToken
	authHeaderLoadConfig = auth.LoadConfig
)

const accessTokenRefreshThreshold = 2 * time.Minute

var authHeaderCmd = &cobra.Command{
	Use:           "auth-header",
	Short:         "Print MCP auth headers as JSON for .mcp.json headersHelper",
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          runAuthHeader,
}

func runAuthHeader(cmd *cobra.Command, args []string) error {
	serverURL := resolveAuthServerURL(cmd)
	if serverURL == "" {
		return fmt.Errorf("--server-url, QUORUM_SERVER_URL, or quorum setup must provide a server URL")
	}
	tokenURL, clientID, err := resolveOAuthEndpoint(cmd, serverURL)
	if err != nil {
		return err
	}
	tokens, err := authHeaderLoadToken(serverURL)
	if err != nil {
		return fmt.Errorf("not logged in (run `flmnt login`): %w", err)
	}
	fresh, refreshed, err := auth.EnsureFreshAccessToken(tokens, tokenURL, clientID, accessTokenRefreshThreshold, time.Now())
	if err != nil {
		return err
	}
	if refreshed {
		if err := authHeaderStoreToken(serverURL, fresh); err != nil {
			return fmt.Errorf("storing refreshed token: %w", err)
		}
	}

	headers := map[string]string{"Authorization": "Bearer " + fresh.AccessToken}
	if wsID := resolveActiveWorkspace(cmd); wsID != "" {
		headers["X-Workspace-Id"] = wsID
	}
	out, err := json.Marshal(headers)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func resolveAuthServerURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("server-url"); v != "" {
		return v
	}
	if v := envOr("QUORUM_SERVER_URL", ""); v != "" {
		return v
	}
	if pc, err := setup.LoadProjectConfig(""); err == nil {
		return pc.ServerURL
	}
	return ""
}

func resolveActiveWorkspace(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("workspace"); v != "" {
		return v
	}
	if cfg, err := authHeaderLoadConfig(); err == nil {
		return cfg.ActiveWorkspaceID
	}
	return ""
}

func resolveOAuthEndpoint(cmd *cobra.Command, serverURL string) (tokenURL, clientID string, err error) {
	tokenURL, _ = cmd.Flags().GetString("token-url")
	if tokenURL == "" {
		tokenURL = envOr("QUORUM_TOKEN_URL", "")
	}
	clientID, _ = cmd.Flags().GetString("client-id")
	if clientID == "" {
		clientID = envOr("QUORUM_CLIENT_ID", "")
	}
	if tokenURL == "" || clientID == "" {
		if cfg, cerr := authHeaderLoadConfig(); cerr == nil {
			if tokenURL == "" {
				tokenURL = cfg.TokenURL
			}
			if clientID == "" {
				clientID = cfg.ClientID
			}
		}
	}
	if tokenURL == "" {
		te, derr := discoverTokenEndpoint(serverURL)
		if derr != nil {
			return "", "", fmt.Errorf("could not resolve token endpoint: %w", derr)
		}
		tokenURL = te
	}
	if clientID == "" {
		return "", "", fmt.Errorf("client id required (--client-id, QUORUM_CLIENT_ID, or `flmnt login`)")
	}
	return tokenURL, clientID, nil
}

func discoverTokenEndpoint(serverURL string) (string, error) {
	doc, err := discoverOAuth(serverURL)
	if err != nil {
		return "", err
	}
	if doc.TokenEndpoint == "" {
		return "", fmt.Errorf("discovery doc has no token_endpoint")
	}
	return doc.TokenEndpoint, nil
}

func init() {
	authHeaderCmd.Flags().String("server-url", "", "Quorum/MCP server URL")
	authHeaderCmd.Flags().String("token-url", "", "OAuth2 token endpoint (default: from MCP discovery)")
	authHeaderCmd.Flags().String("client-id", "", "OAuth2 client ID")
	authHeaderCmd.Flags().String("workspace", "", "Workspace id (default: active workspace from `flmnt workspace use`)")
	mcpCmd.AddCommand(authHeaderCmd)
}
