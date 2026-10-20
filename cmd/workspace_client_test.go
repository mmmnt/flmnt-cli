package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

/*
`flmnt workspace list` authenticates through the same bearer path as brief and derive: the keychain

	token stored under the server URL. For a loopback server that is the one `flmnt login --local`
	stores, so the harness lists its workspaces without a hosted `flmnt login`.
*/
func TestWorkspaceClientAuthenticatesFromTheKeychainForALoopbackServer(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":{"me":{"workspaces":[]}}}`))
	}))
	defer srv.Close()
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", srv.URL)
	token := freshJWT(t, "local-login")
	stubKeychain(t, token, nil)
	if err := workspaceListCmd.Flags().Set("server-url", "http://localhost:8000/mcp"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspaceListCmd.Flags().Set("server-url", "") })

	client, err := workspaceClient(workspaceListCmd)
	if err != nil {
		t.Fatalf("workspaceClient: %v", err)
	}
	if _, err := myWorkspaces(client); err != nil {
		t.Fatal(err)
	}
	if seen != "Bearer "+token {
		t.Fatalf("the router saw %q, want the keychain token", seen)
	}
}

/*
A server with no stored token is refused the same way, local and hosted alike, and the hint names the

	login that fills the keychain for THAT server: `flmnt login --local` for the harness — the old hint
	pointed at a helper flag for a local server — and `flmnt login` for a hosted one.
*/
func TestWorkspaceClientRefusalNamesTheLoginForTheServer(t *testing.T) {
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", "http://127.0.0.1:1/graphql")
	stubKeychain(t, "", auth.ErrNotFound)
	t.Cleanup(func() { _ = workspaceListCmd.Flags().Set("server-url", "") })

	cases := map[string]string{
		"http://localhost:8000/mcp":           "not logged in to http://localhost:8000/mcp: run `flmnt login --local --server-url http://localhost:8000/mcp`",
		"https://mcp.production.flmnt.ai/mcp": "not logged in to https://mcp.production.flmnt.ai/mcp: run `flmnt login --server-url https://mcp.production.flmnt.ai/mcp`",
	}
	for server, want := range cases {
		if err := workspaceListCmd.Flags().Set("server-url", server); err != nil {
			t.Fatal(err)
		}
		_, err := workspaceClient(workspaceListCmd)
		if err == nil || err.Error() != want {
			t.Errorf("%s: got %v, want %q", server, err, want)
		}
	}
}
