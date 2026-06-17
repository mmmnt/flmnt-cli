package browser

import (
	"os"
	"os/exec"
	"runtime"
)

func ResolveURL() string {
	if v := os.Getenv("QUORUM_DASHBOARD_URL"); v != "" {
		return v
	}
	return "http://localhost:3001"
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
