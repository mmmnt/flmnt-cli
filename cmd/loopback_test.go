package cmd

import "testing"

/*
The harness is the server at http://localhost:8000/mcp and the hosted MCP is https://mcp.production.flmnt.ai;

	which one a command talks to decides whether credentials come from the checkout's headers-helper
	or from the keychain `flmnt login` filled. The question is asked of the URL, the same way
	scripts/env.sh asks it of the .mcp.json entry.
*/
func TestIsLoopbackNamesTheLocalHosts(t *testing.T) {
	for _, u := range []string{"http://localhost:8000/mcp", "http://127.0.0.1:8000", "http://[::1]:8000/mcp", "http://localhost"} {
		if !isLoopback(u) {
			t.Errorf("%s should be loopback", u)
		}
	}
	for _, u := range []string{"https://mcp.production.flmnt.ai/mcp", "https://mcp.staging.flmnt.dev", "", "http://flmnt-mcp:8000"} {
		if isLoopback(u) {
			t.Errorf("%s should not be loopback", u)
		}
	}
}
