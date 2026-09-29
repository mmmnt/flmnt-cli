package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

/*
Outside a configured repo, with a perfectly good login on disk, every command that needs a server URL

	refused: whoami answered "--server-url or QUORUM_SERVER_URL is required" and dashboard silently
	opened the local stack. The login config records server_url — resolveActiveWorkspace right beside
	this already falls back to that same file for the workspace, so the two halves of "where am I"
	were resolving from different places. A repo still wins; this only turns a refusal into an answer.
*/
func TestServerURLFallsBackToTheLoginAlreadyRecorded(t *testing.T) {
	orig := authHeaderLoadConfig
	defer func() { authHeaderLoadConfig = orig }()
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp"}, nil
	}
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")

	if got := resolveAuthServerURL(c); got != "https://mcp.production.flmnt.ai/mcp" {
		t.Errorf("with no repo and no flag, the recorded login is the answer; got %q", got)
	}
}

/* A repo's own declaration still outranks the machine-wide login — the v1.10.0 behaviour. */
func TestARepoStillOutranksTheRecordedLogin(t *testing.T) {
	orig := authHeaderLoadConfig
	defer func() { authHeaderLoadConfig = orig }()
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp"}, nil
	}
	t.Setenv("QUORUM_SERVER_URL", "")
	dir := t.TempDir()
	cfg := `{"server_url":"http://localhost:8000","project_id":"ws-1"}`
	if err := os.WriteFile(filepath.Join(dir, ".quorum.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")

	if got := resolveAuthServerURL(c); got != "http://localhost:8000" {
		t.Errorf("the repo's own server URL must win; got %q", got)
	}
}

/*
sync's remote URL resolves through the same chain, so the login fallback reaches it too. Locked before

	removing the copy of that fallback sync.go carried: once resolveAuthServerURL consults the login
	config, a second identical lookup below it can only ever return the same empty string.
*/
func TestSyncsRemoteURLAlsoFallsBackToTheRecordedLogin(t *testing.T) {
	orig := authHeaderLoadConfig
	defer func() { authHeaderLoadConfig = orig }()
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp"}, nil
	}
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")
	c.Flags().String("remote-url", "", "")

	if got := resolveRemoteServerURL(c); got != "https://mcp.production.flmnt.ai/mcp" {
		t.Errorf("sync must reach the recorded login too; got %q", got)
	}
}
