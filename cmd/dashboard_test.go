package cmd_test

import (
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/browser"
)

func TestDashboardPrefersAnExplicitURL(t *testing.T) {
	t.Setenv("QUORUM_DASHBOARD_URL", "http://custom.example.com")

	if url := browser.DashboardURL("https://mcp.production.flmnt.ai"); url != "http://custom.example.com" {
		t.Errorf("an explicit override must win; got %s", url)
	}
}

func TestDashboardFallsBackToTheLocalStackWhenNothingSaysOtherwise(t *testing.T) {
	t.Setenv("QUORUM_DASHBOARD_URL", "")

	if url := browser.DashboardURL(""); url != "http://localhost:3001" {
		t.Errorf("with no login to derive from, the local stack is the honest default; got %s", url)
	}
}
