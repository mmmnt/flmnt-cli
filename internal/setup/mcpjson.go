package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmmnt/flmnt-cli/internal/httpx"
)

// MCPServer is the `flmnt` entry of a checkout's .mcp.json as the CLI reads it: the MCP the session
// talks to and, when the checkout mints its own credentials, the headers-helper that prints them.
type MCPServer struct {
	Type          string `json:"type"`
	URL           string `json:"url"`
	HeadersHelper string `json:"headersHelper,omitempty"`
}

// FlmntEntry reads the `flmnt` server entry from <dir>/.mcp.json.
func FlmntEntry(dir string) (MCPServer, error) {
	raw, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		return MCPServer{}, err
	}
	var file struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return MCPServer{}, err
	}
	entry, ok := file.McpServers[directServerName]
	if !ok {
		return MCPServer{}, fmt.Errorf(".mcp.json has no %q server", directServerName)
	}
	var server MCPServer
	if err := json.Unmarshal(entry, &server); err != nil {
		return MCPServer{}, err
	}
	return server, nil
}

// KeepsLocalEntry says whether setup leaves the checkout's `flmnt` entry alone: a loopback entry that
// carries a headersHelper is the local harness's own declaration — scripts/env.sh reads its host to
// resolve the environment and the helper mints the harness credentials — so rewriting it to the
// hosted URL would move the session and drop the helper in one step.
func KeepsLocalEntry(dir string) bool {
	entry, err := FlmntEntry(dir)
	return err == nil && entry.HeadersHelper != "" && httpx.IsLoopback(entry.URL)
}
