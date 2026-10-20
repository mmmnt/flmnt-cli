package cmd

import (
	"fmt"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/spf13/cobra"
)

// cliTool is the CLI's own OAuth client id on the local broker: the device grant `flmnt login --local`
// runs, and the tool `flmnt local approve` derives the client id for when the pending sign-in is the
// CLI's own. Without it the local token would be another client's and every entry the CLI writes
// would be attributed to that client.
const cliTool = "flmnt-cli"

// loginRequired is the refusal for a server with no stored token. The keychain is filled per server
// URL, so the hint names the login that fills it for THIS one: `flmnt login --local` for the harness,
// `flmnt login` for a hosted flmnt.
func loginRequired(serverURL string) error {
	login := "flmnt login"
	if httpx.IsLoopback(serverURL) {
		login = "flmnt login --local"
	}
	return fmt.Errorf("not logged in to %s: run `%s --server-url %s`", serverURL, login, serverURL)
}

// bestEffortBearer returns a fresh "Bearer <token>" if logged in to serverURL, else "". The keychain
// is the one credential, a loopback server and a hosted one alike: `flmnt login --local` stores the
// harness token under the local server URL exactly as `flmnt login` stores the hosted one. Nothing is
// ever substituted when it holds none. Mirrors how `sync`/`mcp auth-header` mint the header.
func bestEffortBearer(cmd *cobra.Command, serverURL string) string {
	tokens, err := authHeaderLoadToken(serverURL)
	if err != nil {
		return ""
	}
	tokenURL, clientID, err := resolveOAuthEndpoint(cmd, serverURL)
	if err != nil {
		return ""
	}
	fresh, refreshed, err := auth.EnsureFreshAccessToken(tokens, tokenURL, clientID, accessTokenRefreshThreshold, time.Now())
	if err != nil {
		return ""
	}
	if refreshed {
		_ = authHeaderStoreToken(serverURL, fresh)
	}
	return "Bearer " + fresh.AccessToken
}
