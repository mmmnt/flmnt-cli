package cmd_test

import (
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/browser"
)

func TestDashboardResolvesEnvURL(t *testing.T) {
	t.Setenv("QUORUM_DASHBOARD_URL", "http://custom.example.com")
	url := browser.ResolveURL()
	if url != "http://custom.example.com" {
		t.Errorf("expected custom URL, got %s", url)
	}
}

func TestDashboardFallsBackToDefault(t *testing.T) {
	t.Setenv("QUORUM_DASHBOARD_URL", "")
	url := browser.ResolveURL()
	if url != "http://localhost:3001" {
		t.Errorf("expected default URL, got %s", url)
	}
}
