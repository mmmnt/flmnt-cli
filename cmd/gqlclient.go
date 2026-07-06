package cmd

import (
	"fmt"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/spf13/cobra"
)

// graphQLClientFor builds an authenticated router GraphQL client for a server URL: it resolves the
// GraphQL endpoint (flag, env, MCP discovery's graphql_endpoint, then serverURL/graphql) and attaches
// a fresh bearer token. This is the single authenticated path every read/write command uses so the
// router — not the CLI — enforces the session and the caller's rights to the target workspace.
func graphQLClientFor(cmd *cobra.Command, serverURL string) (*apiclient.Client, error) {
	flag, _ := cmd.Flags().GetString("graphql-endpoint")
	env := envOr("FILAMENT_GRAPHQL_ENDPOINT", "")
	discovered := ""
	if flag == "" && env == "" && serverURL != "" {
		if doc, derr := discoverOAuth(serverURL); derr == nil {
			discovered = doc.GraphqlEndpoint
		}
	}
	endpoint := resolveGraphQLEndpoint(flag, env, discovered, serverURL)
	if endpoint == "" {
		return nil, fmt.Errorf("no GraphQL endpoint: pass --graphql-endpoint, set FILAMENT_GRAPHQL_ENDPOINT, or a server URL")
	}
	token := strings.TrimPrefix(bestEffortBearer(cmd, serverURL), "Bearer ")
	return apiclient.New(endpoint, token), nil
}
