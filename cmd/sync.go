package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/sync"
	"github.com/spf13/cobra"
)

const (
	defaultLocalURL       = "http://localhost:8000"
	defaultLocalWorkspace = "quorum"
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

// resolveRemoteServerURL is the hosted MCP URL: --remote-url first, then the chain every other
// command shares (--server-url / QUORUM_SERVER_URL / this repo's config / the login config that
// `flmnt login` writes). sync used to repeat that last fallback itself; once the shared chain
// consults the login config, a second identical lookup can only return the same empty string.
func resolveRemoteServerURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("remote-url"); v != "" {
		return v
	}
	return resolveAuthServerURL(cmd)
}

// resolveRemoteEndpoint resolves the OAuth-authenticated remote side from the active login,
// refreshing the access token the same way `mcp auth-header` does.
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

// resolveLocalEndpoint resolves the local MCP side. Its bearer is the keychain token `flmnt login
// --local` stored under the local server URL, read and refreshed exactly as the remote side's is; a
// local server with no stored token is refused with the login that fills it, never substituted.
func resolveLocalEndpoint(cmd *cobra.Command) (sync.Endpoint, error) {
	url, _ := cmd.Flags().GetString("local-url")
	if url == "" {
		url = defaultLocalURL
	}
	bearer := bestEffortBearer(cmd, url)
	if bearer == "" {
		return sync.Endpoint{}, loginRequired(url)
	}
	workspace, _ := cmd.Flags().GetString("local-workspace")
	endpoint := resolveGraphQLEndpointFor(cmd, url)
	if endpoint == "" {
		return sync.Endpoint{}, fmt.Errorf("no local GraphQL endpoint for %s", url)
	}
	return sync.Endpoint{GQL: apiclient.New(endpoint, strings.TrimPrefix(bearer, "Bearer ")), Workspace: workspace, Ref: url}, nil
}

func init() {
	for _, c := range []*cobra.Command{syncPushCmd, syncPullCmd} {
		c.Flags().Bool("dry-run", false, "Show what would be moved without writing to the target")
		c.Flags().String("server-url", "", "flmnt server URL (default: login config / QUORUM_SERVER_URL); --remote-url wins over it")
		c.Flags().String("remote-url", "", "Remote MCP server URL (default: active login)")
		c.Flags().String("remote-workspace", "", "Remote workspace id (default: active workspace)")
		c.Flags().String("local-url", defaultLocalURL, "Local MCP server URL")
		c.Flags().String("local-workspace", defaultLocalWorkspace, "Local workspace id")
		c.Flags().String("cursor-file", "", "Path to the sync cursor file (default: ~/.filament/sync-cursors.json)")
	}
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncPullCmd)
	rootCmd.AddCommand(syncCmd)
}
