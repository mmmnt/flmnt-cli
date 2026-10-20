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
	stores, so the harness lists its workspaces with no helper in the way.
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
	stubHelper(t, helperPrinting(t, "HELPER-TOKEN"))
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

/* With neither a login nor a helper the refusal still names `flmnt login`. */
func TestWorkspaceClientStillRefusesWithoutAnyCredential(t *testing.T) {
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", "http://127.0.0.1:1/graphql")
	stubKeychain(t, "", auth.ErrNotFound)
	stubHelper(t, helper("exit 1"))
	if err := workspaceListCmd.Flags().Set("server-url", "https://mcp.production.flmnt.ai/mcp"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspaceListCmd.Flags().Set("server-url", "") })

	if _, err := workspaceClient(workspaceListCmd); err == nil {
		t.Fatal("expected a refusal")
	}
}
