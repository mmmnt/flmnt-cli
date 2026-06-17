package cmd

import (
	"fmt"
	"os/exec"

	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure Claude Code integration for Quorum",
	Long: `Writes .mcp.json (pointing to local proxy) and .claude/settings.local.json
(UserPromptSubmit keyframe gate hook). Idempotent — safe to run multiple times.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString("server-url")
		proxyPort, _ := cmd.Flags().GetInt("proxy-port")

		gateCmd, err := resolveGateCmd()
		if err != nil {
			return fmt.Errorf("cannot locate quorum binary: %w", err)
		}

		cfg := setup.Config{
			ServerURL: serverURL,
			ProxyPort: proxyPort,
			GateCmd:   gateCmd + " gate",
		}

		if err := setup.Run(cfg); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Setup complete.\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  .mcp.json          → http://localhost:%d/mcp\n", proxyPort)
		fmt.Fprintf(cmd.OutOrStdout(), "  settings.local.json → UserPromptSubmit: %s gate\n", gateCmd)
		return nil
	},
}

func resolveGateCmd() (string, error) {
	path, err := exec.LookPath("quorum")
	if err != nil {
		return "quorum", nil // fall back to bare name; PATH may differ at hook runtime
	}
	return path, nil
}

func init() {
	setupCmd.Flags().String("server-url", "", "Quorum server URL (required)")
	setupCmd.Flags().Int("proxy-port", 9876, "Local proxy port")
	_ = setupCmd.MarkFlagRequired("server-url")
	rootCmd.AddCommand(setupCmd)
}
