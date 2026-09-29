package browser

import "testing"

func TestDashboardURLDerivesTheEnvironmentFromTheServerURL(t *testing.T) {
	t.Setenv("QUORUM_DASHBOARD_URL", "")

	if got := DashboardURL("https://mcp.production.flmnt.ai/mcp"); got != "https://production.flmnt.ai" {
		t.Errorf("signed in to production, the dashboard is production's; got %q", got)
	}
}
