package cmd

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
)

func serverURLTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	orig := authHeaderLoadConfig
	t.Cleanup(func() { authHeaderLoadConfig = orig })
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	t.Chdir(t.TempDir())
	c := &cobra.Command{}
	c.Flags().String("server-url", "", "")
	return c
}

func TestServerURLFromFlmntServerURLWinsOverQuorumServerURL(t *testing.T) {
	c := serverURLTestCmd(t)
	t.Setenv("FLMNT_SERVER_URL", "http://flmnt.example/mcp")
	t.Setenv("QUORUM_SERVER_URL", "http://quorum.example/mcp")
	if got := resolveAuthServerURL(c); got != "http://flmnt.example/mcp" {
		t.Fatalf("got %q", got)
	}
}

func TestServerURLFromQuorumServerURLAloneStillResolves(t *testing.T) {
	c := serverURLTestCmd(t)
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "http://quorum.example/mcp")
	if got := resolveAuthServerURL(c); got != "http://quorum.example/mcp" {
		t.Fatalf("got %q", got)
	}
}

func TestNotConnectedLineForAHostNamesFlmntServerURL(t *testing.T) {
	line := notConnectedLine(&url.Error{Op: "Post", URL: "https://flmnt.example/graphql", Err: errors.New("refused")})
	if !strings.Contains(line, "FLMNT_SERVER_URL") || strings.Contains(line, "QUORUM") {
		t.Fatalf("got %q", line)
	}
}
