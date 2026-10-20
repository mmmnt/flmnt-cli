package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/mmmnt/flmnt-cli/internal/health"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health of the flmnt server this session talks to",
	Long: "Probes /health on the configured server — the local harness when the server URL is loopback,\n" +
		"the hosted MCP otherwise — and says which environment that is. CORE_URL, ENGINE_URL and\n" +
		"QUORUM_PROXY_PORT add probes for the older host-port local stack only when they are set.",
	Run: func(cmd *cobra.Command, args []string) {
		serverURL := resolveAuthServerURL(cmd)
		cfg := health.Config{
			ServerURL: serverURL,
			CoreURL:   os.Getenv("CORE_URL"),
			EngineURL: os.Getenv("ENGINE_URL"),
		}
		if port := os.Getenv("QUORUM_PROXY_PORT"); port != "" {
			cfg.ProxyURL = "http://localhost:" + port
		}
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, HealthEnvironmentLine(serverURL))
		if !RenderHealth(out, health.Check(cfg)) {
			os.Exit(1)
		}
	},
}

// HealthEnvironmentLine says which server the probe is about, in the words scripts/env.sh uses.
func HealthEnvironmentLine(serverURL string) string {
	switch {
	case serverURL == "":
		return "environment  unresolved — no server URL (flmnt login, QUORUM_SERVER_URL, --server-url or flmnt setup)"
	case httpx.IsLoopback(serverURL):
		return fmt.Sprintf("environment  local (%s)", serverURL)
	default:
		return fmt.Sprintf("environment  hosted (%s)", serverURL)
	}
}

func RenderHealth(w io.Writer, results []health.Result) bool {
	allOK, localDown := true, false
	for _, r := range results {
		if !r.OK {
			allOK = false
			localDown = localDown || r.Service != "mcp"
		}
	}
	if localDown {
		fmt.Fprintln(w, "local stack — hosted flmnt and its MCP tools do not depend on these:")
	}
	for _, r := range results {
		status := "ok"
		if !r.OK {
			status = "down — " + r.Message
		}
		fmt.Fprintf(w, "%-10s %s\n", r.Service, status)
	}
	return allOK
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func init() {
	healthCmd.Flags().String("server-url", "", "flmnt server URL (default: QUORUM_SERVER_URL / this repo's config / login config)")
	rootCmd.AddCommand(healthCmd)
}
