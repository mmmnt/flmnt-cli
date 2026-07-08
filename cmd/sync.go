package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/mmmnt/flmnt-cli/internal/sync"
	"github.com/spf13/cobra"
)

// resolveAuthCmd chooses the local auth-header command. An explicit --local-auth-cmd is honored
// verbatim; the repo-relative default (defaultLocalAuthCmd) runs ONLY inside a configured flmnt
// project (a .quorum.json in the working directory), so `flmnt sync` in an unrelated repo cannot
// auto-execute that repo's script via the SessionEnd hook.
func resolveAuthCmd(flagVal, dir string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if _, err := setup.LoadProjectConfig(dir); err != nil {
		return "", fmt.Errorf("no --local-auth-cmd and no .quorum.json in the working directory; refusing to run the default cwd script %q (run `flmnt setup` here or pass --local-auth-cmd)", defaultLocalAuthCmd)
	}
	return defaultLocalAuthCmd, nil
}

const (
	defaultLocalURL       = "http://localhost:8000"
	defaultLocalWorkspace = "quorum"
	defaultLocalAuthCmd   = "bash scripts/mcp-auth-header.sh"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync your data between local and remote workspaces",
	Long: "Move your data between local and remote workspaces. push sends local to remote; " +
		"pull brings remote to local. Transfers are incremental and safe to re-run.",
}

var syncPushCmd = &cobra.Command{
	Use:           "push",
	Short:         "Sync your local data up to the remote workspace",
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          func(cmd *cobra.Command, args []string) error { return runSync(cmd, true) },
}

var syncPullCmd = &cobra.Command{
	Use:           "pull",
	Short:         "Sync data from the remote workspace down to local",
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          func(cmd *cobra.Command, args []string) error { return runSync(cmd, false) },
}

func runSync(cmd *cobra.Command, push bool) error {
	local, err := resolveLocalEndpoint(cmd)
	if err != nil {
		return fmt.Errorf("resolving local endpoint: %w", err)
	}
	remote, err := resolveRemoteEndpoint(cmd)
	if err != nil {
		return fmt.Errorf("resolving remote endpoint: %w", err)
	}

	from, to := local, remote
	direction := "local -> remote"
	if !push {
		from, to = remote, local
		direction = "remote -> local"
	}

	cursorPath, _ := cmd.Flags().GetString("cursor-file")
	if cursorPath == "" {
		cursorPath, err = sync.DefaultCursorPath()
		if err != nil {
			return err
		}
	}
	cursors, err := sync.LoadCursors(cursorPath)
	if err != nil {
		return fmt.Errorf("loading cursors: %w", err)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	fmt.Fprintf(cmd.OutOrStdout(), "Syncing %s (%s [%s] -> %s [%s])\n",
		direction, from.Ref, from.Workspace, to.Ref, to.Workspace)
	return sync.Run(from, to, cursors, dryRun, cmd.OutOrStdout())
}

// resolveRemoteEndpoint resolves the OAuth-authenticated (staging) side from the
// resolveRemoteServerURL resolves the staging MCP URL using the same precedence
// as the workspace commands: --remote-url, then the shared chain (--server-url /
// QUORUM_SERVER_URL / project config), then the login config that `flmnt login`
// writes (~/.filament/config.json). Without the last fallback, sync would error
// after a plain `flmnt login`.
func resolveRemoteServerURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("remote-url"); v != "" {
		return v
	}
	if v := resolveAuthServerURL(cmd); v != "" {
		return v
	}
	if cfg, err := authHeaderLoadConfig(); err == nil {
		return cfg.ServerURL
	}
	return ""
}

// active login, refreshing the access token the same way `mcp auth-header` does.
func resolveRemoteEndpoint(cmd *cobra.Command) (sync.Endpoint, error) {
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return sync.Endpoint{}, fmt.Errorf("no remote server URL (run `flmnt login` or pass --remote-url)")
	}
	tokenURL, clientID, err := resolveOAuthEndpoint(cmd, serverURL)
	if err != nil {
		return sync.Endpoint{}, err
	}
	tokens, err := authHeaderLoadToken(serverURL)
	if err != nil {
		return sync.Endpoint{}, fmt.Errorf("not logged in to %s (run `flmnt login`): %w", serverURL, err)
	}
	fresh, refreshed, err := auth.EnsureFreshAccessToken(tokens, tokenURL, clientID, accessTokenRefreshThreshold, time.Now())
	if err != nil {
		return sync.Endpoint{}, err
	}
	if refreshed {
		if err := authHeaderStoreToken(serverURL, fresh); err != nil {
			return sync.Endpoint{}, fmt.Errorf("storing refreshed token: %w", err)
		}
	}

	workspace := resolveActiveWorkspace(cmd)
	if v, _ := cmd.Flags().GetString("remote-workspace"); v != "" {
		workspace = v
	}
	if workspace == "" {
		return sync.Endpoint{}, fmt.Errorf("no remote workspace (run `flmnt workspace use` or pass --remote-workspace)")
	}
	endpoint := resolveGraphQLEndpointFor(cmd, serverURL)
	if endpoint == "" {
		return sync.Endpoint{}, fmt.Errorf("no remote GraphQL endpoint for %s", serverURL)
	}
	return sync.Endpoint{GQL: apiclient.New(endpoint, fresh.AccessToken), Workspace: workspace, Ref: serverURL}, nil
}

// resolveLocalEndpoint resolves the local MCP side. Its auth header comes from a
// headers-helper command (the same mechanism .mcp.json uses), so we never need
// to know how the local stack mints tokens.
func resolveLocalEndpoint(cmd *cobra.Command) (sync.Endpoint, error) {
	url, _ := cmd.Flags().GetString("local-url")
	if url == "" {
		url = defaultLocalURL
	}
	workspace, _ := cmd.Flags().GetString("local-workspace")
	if workspace == "" {
		workspace = defaultLocalWorkspace
	}
	authCmdFlag, _ := cmd.Flags().GetString("local-auth-cmd")
	authCmd, err := resolveAuthCmd(authCmdFlag, "")
	if err != nil {
		return sync.Endpoint{}, err
	}
	authValue, err := runAuthHelper(authCmd)
	if err != nil {
		return sync.Endpoint{}, fmt.Errorf("local auth (%q): %w", authCmd, err)
	}
	endpoint := resolveGraphQLEndpointFor(cmd, url)
	if endpoint == "" {
		return sync.Endpoint{}, fmt.Errorf("no local GraphQL endpoint for %s", url)
	}
	return sync.Endpoint{GQL: apiclient.New(endpoint, strings.TrimPrefix(authValue, "Bearer ")), Workspace: workspace, Ref: url}, nil
}

// runAuthHelper runs a headers-helper command and extracts the Authorization
// header from its JSON stdout (e.g. {"Authorization": "Bearer ..."}).
func runAuthHelper(command string) (string, error) {
	out, err := exec.Command("sh", "-c", command).Output()
	if err != nil {
		return "", err
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &headers); err != nil {
		return "", fmt.Errorf("auth helper did not print JSON headers: %w", err)
	}
	for k, v := range headers {
		if strings.EqualFold(k, "Authorization") {
			return v, nil
		}
	}
	return "", fmt.Errorf("auth helper output has no Authorization header")
}

func init() {
	for _, c := range []*cobra.Command{syncPushCmd, syncPullCmd} {
		c.Flags().Bool("dry-run", false, "Show what would be moved without writing to the target")
		c.Flags().String("remote-url", "", "Remote MCP server URL (default: active login)")
		c.Flags().String("remote-workspace", "", "Remote workspace id (default: active workspace)")
		c.Flags().String("local-url", defaultLocalURL, "Local MCP server URL")
		c.Flags().String("local-workspace", defaultLocalWorkspace, "Local workspace id")
		c.Flags().String("local-auth-cmd", "", "Command that prints local MCP auth headers as JSON (defaults to "+defaultLocalAuthCmd+" only inside a configured flmnt project)")
		c.Flags().String("cursor-file", "", "Path to the sync cursor file (default: ~/.filament/sync-cursors.json)")
	}
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncPullCmd)
	rootCmd.AddCommand(syncCmd)
}
