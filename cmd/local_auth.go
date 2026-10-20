package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// The CLI's own registered client id — the one its hosted writes carry. A headers-helper mints the
// local credential for the client id it is given, so the CLI declares its own; without this the local
// token would be Claude Code's and every entry the CLI writes would be attributed to Claude Code.
const (
	cliClientID       = "flmnt-cli"
	helperClientIDVar = "BROKER_CLIENT_ID"
)

// helperHeaders is what a headers-helper prints: the bearer (without the "Bearer " prefix) and the
// workspace it scopes the session to, when it prints one.
type helperHeaders struct {
	Bearer    string
	Workspace string
}

// runAuthHelper runs a headers-helper command (the same mechanism .mcp.json uses) and reads the
// Authorization and X-Workspace-Id headers from its JSON stdout. A helper that refuses exits non-zero
// with its reason on stderr; that reason is the error, and nothing is substituted for it.
func runAuthHelper(command string) (helperHeaders, error) {
	c := exec.Command("sh", "-c", command)
	c.Env = append(os.Environ(), helperClientIDVar+"="+cliClientID)
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
