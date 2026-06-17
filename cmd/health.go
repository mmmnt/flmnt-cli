package cmd

import (
	"fmt"
	"os"

	"github.com/mmmnt/flmnt-cli/internal/health"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check health of Core, Engine, and proxy services",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := health.Config{
			CoreURL:   envOr("CORE_URL", "http://localhost:3000"),
			EngineURL: envOr("ENGINE_URL", "http://localhost:3001"),
			ProxyURL:  fmt.Sprintf("http://localhost:%s", envOr("QUORUM_PROXY_PORT", "9876")),
		}
		results := health.Check(cfg)
		allOK := true
		for _, r := range results {
			status := "ok"
			if !r.OK {
				status = "down — " + r.Message
				allOK = false
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s\n", r.Service, status)
		}
		if !allOK {
			os.Exit(1)
		}
	},
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
