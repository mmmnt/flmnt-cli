package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/mmmnt/flmnt-cli/internal/health"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health of local flmnt services",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := health.Config{
			CoreURL:   envOr("CORE_URL", "http://localhost:3000"),
			EngineURL: envOr("ENGINE_URL", "http://localhost:3001"),
			ProxyURL:  fmt.Sprintf("http://localhost:%s", envOr("QUORUM_PROXY_PORT", "9876")),
		}
		if !RenderHealth(cmd.OutOrStdout(), health.Check(cfg)) {
			os.Exit(1)
		}
	},
}

func RenderHealth(w io.Writer, results []health.Result) bool {
	allOK := true
	for _, r := range results {
		if !r.OK {
			allOK = false
		}
	}
	if !allOK {
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
	rootCmd.AddCommand(healthCmd)
}
