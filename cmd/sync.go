package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/mmmnt/flmnt-cli/internal/localadmin"
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
		"pull brings remote to local. Transfers are incremental and safe to re-run.\n" +
		"A push converges your local identity on your cloud account (QRM-ADR-016): records you wrote\n" +
		"locally are written to the cloud as the account you are logged in to; a teammate's stay theirs.\n" +
		"A pull writes the cloud copy as it is; a record that differs in both places is shown as a diff\n" +
		"and you decide, record by record (or --take local|cloud for all). A record you keep is\n" +
		"replaced in the cloud by your next push.",
}

var syncPushCmd = &cobra.Command{
	Use:           "push",
	Short:         "Sync your local data up to the remote workspace, as your cloud account",
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          func(cmd *cobra.Command, args []string) error { return runSync(cmd, true) },
}

var syncPullCmd = &cobra.Command{
	Use:           "pull",
	Short:         "Sync data from the remote workspace down to local, deciding every collision",
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          func(cmd *cobra.Command, args []string) error { return runSync(cmd, false) },
}

func runSync(cmd *cobra.Command, push bool) error {
	local, err := resolveLocalEndpoint(cmd)
	if err != nil {
		return fmt.Errorf("resolving local endpoint: %w", err)
	}
	remote, hosted, err := resolveRemoteEndpoint(cmd)
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
	markPath, _ := cmd.Flags().GetString("marks-file")
	if markPath == "" {
		markPath, err = sync.DefaultMarkPath()
		if err != nil {
			return err
		}
	}
	marks, err := sync.LoadMarks(markPath)
	if err != nil {
		return fmt.Errorf("loading marks: %w", err)
	}

	opts := sync.Options{Marks: marks}
	out := cmd.OutOrStdout()
	if push {
		if id, err := identityLoad(); err == nil {
			opts.LocalSubs = append([]string{id.Sub}, id.Formerly()...)
			fmt.Fprintf(out, "Pushing as %s (%s); your local identity %s is linked to it — records keep their local author.\n", identityOf(hosted), hosted.Sub, id.Sub)
		}
	} else {
		take, _ := cmd.Flags().GetString("take")
		resolver, err := collisionResolver(cmd, take)
		if err != nil {
			return err
		}
		opts.Resolve = resolver
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	fmt.Fprintf(out, "Syncing %s (%s [%s] -> %s [%s])\n",
		direction, from.Ref, from.Workspace, to.Ref, to.Workspace)
	return sync.RunWith(from, to, cursors, dryRun, out, opts)
}

// collisionResolver is how a pull decides (QRM-ADR-016 §2): --take local|cloud decides every collision
// the same way; otherwise each collision is shown as a diff and the person answers l or c. An answer
// that is neither, or no answer at all (no terminal), stops the pull before anything is written.
func collisionResolver(cmd *cobra.Command, take string) (sync.Resolver, error) {
	switch take {
	case "local", "cloud":
		return func(cs []sync.Collision) ([]sync.Resolution, error) {
			out := make([]sync.Resolution, 0, len(cs))
			for _, c := range cs {
				out = append(out, sync.Resolution{CorrelationID: c.CorrelationID, Take: takeOf(take)})
			}
			return out, nil
		}, nil
	case "":
		return func(cs []sync.Collision) ([]sync.Resolution, error) {
			reader := bufio.NewReader(cmd.InOrStdin())
			w := cmd.OutOrStdout()
			out := make([]sync.Resolution, 0, len(cs))
			for i, c := range cs {
				fmt.Fprintf(w, "\n[%d/%d] %s / %s\n%s", i+1, len(cs), c.StreamSuffix, c.CorrelationID, renderDiff(c))
				fmt.Fprint(w, "Keep the (l)ocal copy or take the (c)loud copy? [l/c] ")
				line, _ := reader.ReadString('\n')
				fmt.Fprintln(w)
				switch strings.ToLower(strings.TrimSpace(line)) {
				case "l", "local":
					out = append(out, sync.Resolution{CorrelationID: c.CorrelationID, Take: "EXISTING"})
				case "c", "cloud":
					out = append(out, sync.Resolution{CorrelationID: c.CorrelationID, Take: "INCOMING"})
				default:
					return nil, fmt.Errorf("every collision needs a decision; answer l or c, or pass --take local|cloud (nothing was written)")
				}
			}
			return out, nil
		}, nil
	default:
		return nil, fmt.Errorf("--take must be local or cloud, got %q", take)
	}
}

func takeOf(take string) string {
	if take == "local" {
		return "EXISTING"
	}
	return "INCOMING"
}

// renderDiff names each top-level field of the envelope that differs, local value then cloud value,
// as compact JSON. The envelopes arrive as the server serialized them.
func renderDiff(c sync.Collision) string {
	var local, cloud map[string]json.RawMessage
	_ = json.Unmarshal([]byte(c.Existing), &local)
	_ = json.Unmarshal([]byte(c.Incoming), &cloud)
	keys := map[string]bool{}
	for k := range local {
		keys[k] = true
	}
	for k := range cloud {
		keys[k] = true
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, k := range names {
		if string(local[k]) == string(cloud[k]) {
			continue
		}
		fmt.Fprintf(&b, "  %s\n    local: %s\n    cloud: %s\n", k, valueOrAbsent(local[k]), valueOrAbsent(cloud[k]))
	}
	return b.String()
}

func valueOrAbsent(v json.RawMessage) string {
	if len(v) == 0 {
		return "(absent)"
	}
	return string(v)
}

// resolveRemoteServerURL is the hosted MCP URL: --remote-url first, then the chain every other
// command shares (--server-url / FLMNT_SERVER_URL / this repo's config / the login config that
// `flmnt login` writes). sync used to repeat that last fallback itself; once the shared chain
// consults the login config, a second identical lookup can only return the same empty string.
func resolveRemoteServerURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("remote-url"); v != "" {
		return v
	}
	return resolveAuthServerURL(cmd)
}

// resolveRemoteEndpoint resolves the OAuth-authenticated remote side from the active login,
// refreshing the access token the same way `mcp auth-header` does, and says who the token is.
func resolveRemoteEndpoint(cmd *cobra.Command) (sync.Endpoint, auth.Claims, error) {
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return sync.Endpoint{}, auth.Claims{}, fmt.Errorf("no remote server URL (run `flmnt login` or pass --remote-url)")
	}
	tokenURL, clientID, err := resolveOAuthEndpoint(cmd, serverURL)
	if err != nil {
		return sync.Endpoint{}, auth.Claims{}, err
	}
	tokens, err := authHeaderLoadToken(serverURL)
	if err != nil {
		return sync.Endpoint{}, auth.Claims{}, fmt.Errorf("not logged in to %s (run `flmnt login`): %w", serverURL, err)
	}
	fresh, refreshed, err := auth.EnsureFreshAccessToken(tokens, tokenURL, clientID, accessTokenRefreshThreshold, time.Now())
	if err != nil {
		return sync.Endpoint{}, auth.Claims{}, err
	}
	if refreshed {
		if err := authHeaderStoreToken(serverURL, fresh); err != nil {
			return sync.Endpoint{}, auth.Claims{}, fmt.Errorf("storing refreshed token: %w", err)
		}
	}

	workspace := resolveActiveWorkspace(cmd)
	if v, _ := cmd.Flags().GetString("remote-workspace"); v != "" {
		workspace = v
	}
	if workspace == "" {
		return sync.Endpoint{}, auth.Claims{}, fmt.Errorf("no remote workspace (run `flmnt workspace use` or pass --remote-workspace)")
	}
	endpoint := resolveGraphQLEndpointFor(cmd, serverURL)
	if endpoint == "" {
		return sync.Endpoint{}, auth.Claims{}, fmt.Errorf("no remote GraphQL endpoint for %s", serverURL)
	}
	claims, _ := auth.DecodeUnverified(firstNonEmpty(fresh.IDToken, fresh.AccessToken))
	return sync.Endpoint{GQL: apiclient.New(endpoint, fresh.AccessToken), Workspace: workspace, Ref: serverURL}, claims, nil
}

// resolveLocalEndpoint resolves the local MCP side. Its bearer is the keychain token `flmnt login
// --local` stored under the local server URL, read and refreshed exactly as the remote side's is.
// When the keychain holds none — a hosted login sweeps every token (one login at a time) — and this
// machine holds its local identity, the token is minted again from the identity on the spot
// (QRM-ADR-016 §5f); a machine with no identity is refused with the login that records one.
func resolveLocalEndpoint(cmd *cobra.Command) (sync.Endpoint, error) {
	url, _ := cmd.Flags().GetString("local-url")
	if url == "" {
		url = defaultLocalURL
	}
	bearer := bestEffortBearer(cmd, url)
	if bearer == "" {
		minted, err := mintLocalBearer(cmd, url)
		if err != nil {
			return sync.Endpoint{}, err
		}
		bearer = minted
	}
	workspace, _ := cmd.Flags().GetString("local-workspace")
	endpoint := resolveGraphQLEndpointFor(cmd, url)
	if endpoint == "" {
		return sync.Endpoint{}, fmt.Errorf("no local GraphQL endpoint for %s", url)
	}
	return sync.Endpoint{GQL: apiclient.New(endpoint, strings.TrimPrefix(bearer, "Bearer ")), Workspace: workspace, Ref: url}, nil
}

func mintLocalBearer(cmd *cobra.Command, url string) (string, error) {
	if !httpx.IsLoopback(url) {
		return "", loginRequired(url)
	}
	id, err := identityLoad()
	if err != nil {
		return "", loginRequired(url)
	}
	doc, err := loginDiscover(url)
	if err != nil {
		return "", fmt.Errorf("no stored token for %s and its OAuth discovery failed: %w", url, err)
	}
	if doc.DeviceAuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return "", loginRequired(url)
	}
	adminURL, _ := cmd.Flags().GetString("admin-url")
	tokens, err := obtainLocalToken(id, url, adminURL, doc)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Local token minted for %s (%s).\n", id.Sub, url)
	return "Bearer " + tokens.AccessToken, nil
}

func init() {
	for _, c := range []*cobra.Command{syncPushCmd, syncPullCmd} {
		c.Flags().Bool("dry-run", false, "Show what would be moved without writing to the target")
		c.Flags().String("server-url", "", "flmnt server URL (default: login config / FLMNT_SERVER_URL); --remote-url wins over it")
		c.Flags().String("remote-url", "", "Remote MCP server URL (default: active login)")
		c.Flags().String("remote-workspace", "", "Remote workspace id (default: active workspace)")
		c.Flags().String("local-url", defaultLocalURL, "Local MCP server URL")
		c.Flags().String("local-workspace", defaultLocalWorkspace, "Local workspace id")
		c.Flags().String("cursor-file", "", "Path to the sync cursor file (default: ~/.flmnt/sync-cursors.json)")
		c.Flags().String("marks-file", "", "Path to the file of records kept as local truth (default: ~/.flmnt/sync-marks.json)")
		c.Flags().String("admin-url", localadmin.DefaultBaseURL, "the local broker's admin listener, for minting the local token when the keychain holds none")
	}
	syncPullCmd.Flags().String("take", "", "decide every collision the same way: local keeps your copy, cloud takes the cloud copy")
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncPullCmd)
	rootCmd.AddCommand(syncCmd)
}
