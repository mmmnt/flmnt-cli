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

/*
`flmnt setup --server-url <hosted>` rewrote the `flmnt` entry to {type,url} at the hosted URL. In a

	checkout on the local harness that entry IS the environment declaration (scripts/env.sh reads its
	host) and carries the headers-helper that mints the harness credentials, so re-running setup
	silently moved the session to the hosted side and dropped the helper.
*/
func TestRunKeepsALoopbackFlmntEntryThatCarriesAHeadersHelper(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(quorumMCPJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(Config{ServerURL: "https://mcp.production.flmnt.ai", ProjectID: "p", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entry, err := FlmntEntry(dir)
	if err != nil {
		t.Fatalf("FlmntEntry: %v", err)
	}
	if entry.URL != "http://localhost:8000/mcp" || entry.HeadersHelper != "bash scripts/mcp-auth-header.sh" {
		t.Errorf("the local entry was rewritten: %+v", entry)
	}
	if _, ok := readMCPServers(t, dir)["atlassian"]; !ok {
		t.Error("the other server must survive verbatim")
	}
	if !KeepsLocalEntry(dir) {
		t.Error("setup must be able to say that it kept the entry")
	}
	// The repo config still records the hosted URL the founder asked for: the pin is per-repo truth,
	// the .mcp.json entry is the session's environment, and the two are allowed to differ.
	pc, err := LoadProjectConfig(dir)
	if err != nil || pc.ServerURL != "https://mcp.production.flmnt.ai" {
		t.Errorf("project config: %+v err %v", pc, err)
	}
}

/* A hosted entry with a headersHelper (the aws-mode helper delegating to `flmnt mcp auth-header`) is setup's to rewrite, as before. */
func TestRunStillRewritesAHostedFlmntEntry(t *testing.T) {
	dir := t.TempDir()
	seed := `{"mcpServers":{"flmnt":{"type":"http","url":"https://mcp.staging.flmnt.dev/mcp","headersHelper":"bash scripts/mcp-auth-header.sh"}}}`
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(Config{ServerURL: "https://mcp.production.flmnt.ai", ProjectID: "p", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entry, _ := FlmntEntry(dir)
	if entry.URL != "https://mcp.production.flmnt.ai" || KeepsLocalEntry(dir) {
		t.Errorf("a hosted entry is setup-managed and must follow --server-url: %+v", entry)
	}
}

/*
The harness publishes its router on the host and the checkout DECLARES where, in .quorum.json. setup

	rewrites that file on every run, so a declaration it does not know would be dropped the next time
	anyone re-ran setup; the field is read back and written out.
*/
func TestRunPreservesTheLocalGraphQLEndpointDeclaration(t *testing.T) {
	dir := t.TempDir()
	seed := `{"server_url":"https://mcp.production.flmnt.ai","project_id":"bc674142","local_graphql_endpoint":"http://localhost:4000/graphql"}`
	if err := os.WriteFile(filepath.Join(dir, ".quorum.json"), []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Run(Config{ServerURL: "https://mcp.production.flmnt.ai", ProjectID: "bc674142", ProxyPort: 9876, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	pc, err := LoadProjectConfig(dir)
	if err != nil || pc.LocalGraphQLEndpoint != "http://localhost:4000/graphql" {
		t.Fatalf("got %+v err %v", pc, err)
	}
}
