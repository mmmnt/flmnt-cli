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
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")

	if got := resolveAuthServerURL(c); got != "https://mcp.production.flmnt.ai/mcp" {
		t.Errorf("with no repo and no flag, the recorded login is the answer; got %q", got)
	}
}

/*
The login outranks the repo. Founder ruling 2026-10-20: the CLI holds one login at a time, and
every command takes its upstream from what that login recorded — no --server-url, no env. A repo
pinned to production used to outrank it, which is how `flmnt login --local` in that repo kept
whoami answering with the production account's email.
*/
func TestTheRecordedLoginOutranksTheRepo(t *testing.T) {
	orig := authHeaderLoadConfig
	defer func() { authHeaderLoadConfig = orig }()
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginLocal, ServerURL: "http://localhost:8000"}, nil
	}
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	dir := t.TempDir()
	cfg := `{"server_url":"https://mcp.production.flmnt.ai","project_id":"ws-1"}`
	if err := os.WriteFile(filepath.Join(dir, ".quorum.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")

	if got := resolveAuthServerURL(c); got != "http://localhost:8000" {
		t.Errorf("the login's server must win over the repo's; got %q", got)
	}
}

/* With nothing logged in, the repo's own declaration is the answer, as before. */
func TestTheRepoAnswersWhenNothingIsLoggedIn(t *testing.T) {
	orig := authHeaderLoadConfig
	defer func() { authHeaderLoadConfig = orig }()
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	dir := t.TempDir()
	cfg := `{"server_url":"https://mcp.production.flmnt.ai","project_id":"ws-1"}`
	if err := os.WriteFile(filepath.Join(dir, ".quorum.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")

	if got := resolveAuthServerURL(c); got != "https://mcp.production.flmnt.ai" {
		t.Errorf("got %q", got)
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
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Chdir(t.TempDir())
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")
	c.Flags().String("remote-url", "", "")

	if got := resolveRemoteServerURL(c); got != "https://mcp.production.flmnt.ai/mcp" {
		t.Errorf("sync must reach the recorded login too; got %q", got)
	}
}
