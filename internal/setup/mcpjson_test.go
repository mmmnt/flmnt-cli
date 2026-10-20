package setup

import (
	"os"
	"path/filepath"
	"testing"
)

const quorumMCPJSON = `{
  "mcpServers": {
    "atlassian": {"type": "stdio", "command": "uvx", "args": ["mcp-atlassian"]},
    "flmnt": {
      "type": "http",
      "url": "http://localhost:8000/mcp",
      "headersHelper": "bash scripts/mcp-auth-header.sh"
    }
  }
}`

func TestHeadersHelperReadsTheFlmntEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(quorumMCPJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	entry, err := FlmntEntry(dir)
	if err != nil {
		t.Fatalf("FlmntEntry: %v", err)
	}
	if entry.HeadersHelper != "bash scripts/mcp-auth-header.sh" {
		t.Errorf("headersHelper = %q", entry.HeadersHelper)
	}
	if entry.URL != "http://localhost:8000/mcp" {
		t.Errorf("url = %q", entry.URL)
	}
}

func TestFlmntEntryIsAbsentWithoutAnMCPJSON(t *testing.T) {
	if _, err := FlmntEntry(t.TempDir()); err == nil {
		t.Fatal("expected an error when there is no .mcp.json")
	}
}

func TestFlmntEntryIsAbsentWhenOnlyOtherServersExist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{"mcpServers":{"other":{"type":"http","url":"http://other:8080"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FlmntEntry(dir); err == nil {
		t.Fatal("expected an error when .mcp.json has no flmnt entry")
	}
}
