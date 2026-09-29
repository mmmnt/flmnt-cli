package browser

import (
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// localDashboard is where the dashboard runs in a local stack — the fallback when the environment
// cannot be derived from a login.
const localDashboard = "http://localhost:3001"

// DashboardURL is the dashboard for the environment the caller is signed in to. An explicit
// QUORUM_DASHBOARD_URL still wins. Otherwise the environment is taken from the server URL the rest of
// the CLI already resolves: the MCP host is the dashboard host with an `mcp.` label in front, so
// dropping that label names the app. Anything else — no login, a local stack, an unfamiliar shape —
// falls back to the local stack rather than guessing at a host.
func DashboardURL(serverURL string) string {
	if v := os.Getenv("QUORUM_DASHBOARD_URL"); v != "" {
		return v
	}
	u, err := url.Parse(serverURL)
	if err != nil || u.Scheme == "" || !strings.HasPrefix(u.Host, "mcp.") {
		return localDashboard
	}
	return u.Scheme + "://" + strings.TrimPrefix(u.Host, "mcp.")
}

func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
