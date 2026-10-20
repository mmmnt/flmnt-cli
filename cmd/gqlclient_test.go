package cmd

import (
	"os"
	"testing"
)

/*
A local flmnt has no OAuth discovery, so nothing advertised where its router is and every GraphQL

	command fell to <server>/graphql — the MCP, which has no /graphql. The checkout declares the
	router in .quorum.json (local_graphql_endpoint); on a loopback server that declaration is read
	before the fallback. Flag and env still win, and a hosted server never reads it.
*/
func TestResolveGraphQLEndpointReadsTheCheckoutsLocalDeclaration(t *testing.T) {
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", "")
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.quorum.json", []byte(`{"server_url":"https://mcp.production.flmnt.ai","project_id":"p","local_graphql_endpoint":"http://localhost:4000/graphql"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if got := resolveGraphQLEndpointFor(briefCmd, "http://127.0.0.1:1/mcp"); got != "http://localhost:4000/graphql" {
		t.Fatalf("loopback: got %q, want the declared local router", got)
	}
	if got := resolveGraphQLEndpointFor(briefCmd, "http://127.0.0.1:1/mcp"); got == "" {
		t.Fatal("unreachable")
	}
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", "http://env.example/graphql")
	if got := resolveGraphQLEndpointFor(briefCmd, "http://127.0.0.1:1/mcp"); got != "http://env.example/graphql" {
		t.Fatalf("env must still win: %q", got)
	}
}
