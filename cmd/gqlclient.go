package cmd

import (
	"fmt"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/spf13/cobra"
)

// resolveGraphQLEndpointFor resolves the router GraphQL endpoint for a server URL: an explicit flag or
// env, else the MCP discovery's graphql_endpoint, else serverURL/graphql.
func resolveGraphQLEndpointFor(cmd *cobra.Command, serverURL string) string {
	flag, _ := cmd.Flags().GetString("graphql-endpoint")
	env := envOr("FILAMENT_GRAPHQL_ENDPOINT", "")
	discovered := ""
	if flag == "" && env == "" && serverURL != "" {
		if doc, derr := discoverOAuth(serverURL); derr == nil {
			discovered = doc.GraphqlEndpoint
		}
	}
	return resolveGraphQLEndpoint(flag, env, discovered, serverURL)
}

// graphQLClientFor builds an authenticated router GraphQL client for a server URL, attaching a fresh
// OAuth bearer token. This is the single authenticated path every read/write command uses so the
// router — not the CLI — enforces the session and the caller's rights to the target workspace.
func graphQLClientFor(cmd *cobra.Command, serverURL string) (*apiclient.Client, error) {
	endpoint := resolveGraphQLEndpointFor(cmd, serverURL)
	if endpoint == "" {
		return nil, fmt.Errorf("no GraphQL endpoint: pass --graphql-endpoint, set FILAMENT_GRAPHQL_ENDPOINT, or a server URL")
	}
	token := strings.TrimPrefix(bestEffortBearer(cmd, serverURL), "Bearer ")
	return apiclient.New(endpoint, token), nil
}
