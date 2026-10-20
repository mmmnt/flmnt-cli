package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/spf13/cobra"
)

// cliTool is the name the CLI gives a headers-helper for its own credential (`--client flmnt-cli`);
// the helper derives this person's client id for that tool through `flmnt whoami --json --client`.
// Without it the local token would be Claude Code's and every entry the CLI writes would be
// attributed to Claude Code.
const cliTool = "flmnt-cli"

// helperHeaders is what a headers-helper prints: the bearer (without the "Bearer " prefix) and the
// workspace it scopes the session to, when it prints one.
type helperHeaders struct {
	Bearer    string
	Workspace string
}

// runAuthHelper runs a headers-helper command (the same mechanism .mcp.json uses) for the CLI's own
// tool and reads the Authorization and X-Workspace-Id headers from its JSON stdout. A helper that
// refuses exits non-zero with its reason on stderr; that reason is the error, and nothing is
// substituted for it.
func runAuthHelper(command string) (helperHeaders, error) {
	c := exec.Command("sh", "-c", command+" --client "+cliTool)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if reason := strings.TrimSpace(stderr.String()); reason != "" {
			return helperHeaders{}, fmt.Errorf("%s", reason)
		}
		return helperHeaders{}, err
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &headers); err != nil {
		return helperHeaders{}, fmt.Errorf("auth helper did not print JSON headers: %w", err)
	}
	var h helperHeaders
	for k, v := range headers {
		switch {
		case strings.EqualFold(k, "Authorization"):
			h.Bearer = strings.TrimPrefix(v, "Bearer ")
		case strings.EqualFold(k, "X-Workspace-Id"):
			h.Workspace = v
		}
	}
	if h.Bearer == "" {
		return helperHeaders{}, fmt.Errorf("auth helper output has no Authorization header")
	}
	return h, nil
}

// helperRuns memoizes each helper command's answer for the life of the process: a mint costs
// seconds, and one command asks for the bearer and again for the workspace.
var helperRuns = map[string]helperRun{}

type helperRun struct {
	headers helperHeaders
	err     error
}

// localCredentials resolves and runs the headers-helper for the working directory: --local-auth-cmd
// (sync's local side) or the root --auth-cmd, else the checkout's .mcp.json entry, else the gated
// default. dir "" is the working directory.
func localCredentials(cmd *cobra.Command, dir string) (helperHeaders, error) {
	flag, _ := cmd.Flags().GetString("local-auth-cmd")
	if flag == "" {
		flag, _ = cmd.Root().PersistentFlags().GetString("auth-cmd")
	}
	command, err := resolveAuthCmd(flag, dir)
	if err != nil {
		return helperHeaders{}, err
	}
	run, ok := helperRuns[command]
	if !ok {
		run.headers, run.err = runAuthHelper(command)
		helperRuns[command] = run
	}
	return run.headers, run.err
}

// bestEffortBearer returns "Bearer <token>" for serverURL, or "" when no credential answers. For a
// loopback server the checkout's headers-helper is the credential and is asked first, the keychain
// second; for a hosted server the keychain `flmnt login` filled stays first and the helper is a
// fallback. Nothing is ever substituted when neither answers.
func bestEffortBearer(cmd *cobra.Command, serverURL string) string {
	keychain := func() string { return keychainBearer(cmd, serverURL) }
	helper := func() string {
		h, err := localCredentials(cmd, "")
		if err != nil {
			return ""
		}
		return "Bearer " + h.Bearer
	}
	sources := []func() string{keychain, helper}
	if httpx.IsLoopback(serverURL) {
		sources = []func() string{helper, keychain}
	}
	for _, source := range sources {
		if bearer := source(); bearer != "" {
			return bearer
		}
	}
	return ""
}

// keychainBearer returns a fresh "Bearer <token>" if logged in to serverURL, else "". Mirrors how
// `sync`/`mcp auth-header` mint the header.
func keychainBearer(cmd *cobra.Command, serverURL string) string {
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

func init() {
	rootCmd.PersistentFlags().String("auth-cmd", "", "command that prints MCP auth headers as JSON for a local server (default: the flmnt entry's headersHelper in .mcp.json)")
}
