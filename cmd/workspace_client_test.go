package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

/*
`flmnt workspace list` read the keychain and refused with "not logged in" against the harness, whose

	credential the checkout's headers-helper mints. It authenticates through the same bearer path as
	brief and derive now, so a loopback server lists its workspaces without `flmnt login`.
*/
func TestWorkspaceClientAuthenticatesThroughTheHelperForALoopbackServer(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":{"me":{"workspaces":[]}}}`))
	}))
	defer srv.Close()
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", srv.URL)
	stubKeychain(t, "", auth.ErrNotFound)
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
	if seen != "Bearer HELPER-TOKEN" {
		t.Fatalf("the router saw %q, want the helper's bearer", seen)
	}
}

/* With neither a login nor a helper the refusal still names `flmnt login`. */
func TestWorkspaceClientStillRefusesWithoutAnyCredential(t *testing.T) {
	t.Setenv("FILAMENT_GRAPHQL_ENDPOINT", "http://127.0.0.1:1/graphql")
	stubKeychain(t, "", auth.ErrNotFound)
	stubHelper(t, "exit 1")
	if err := workspaceListCmd.Flags().Set("server-url", "https://mcp.production.flmnt.ai/mcp"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspaceListCmd.Flags().Set("server-url", "") })

	if _, err := workspaceClient(workspaceListCmd); err == nil {
		t.Fatal("expected a refusal")
	}
}
