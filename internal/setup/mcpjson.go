package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
