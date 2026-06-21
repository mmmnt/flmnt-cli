package cmd

import (
	"fmt"
	"os/exec"

	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure Claude Code integration",
	Long: `Writes .mcp.json (pointing to the local proxy) and .claude/settings.local.json
(UserPromptSubmit context hook). Idempotent — safe to run multiple times.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString("server-url")
		proxyPort, _ := cmd.Flags().GetInt("proxy-port")
		project, _ := cmd.Flags().GetString("project")

		flmntCmd, err := resolveGateCmd()
		if err != nil {
			return fmt.Errorf("cannot locate flmnt binary: %w", err)
		}

		cfg := setup.Config{
			ServerURL: serverURL,
			ProjectID: project,
			ProxyPort: proxyPort,
			GateCmd:   flmntCmd + " gate",
			BriefCmd:  flmntCmd + " brief",
			DeriveCmd: flmntCmd + " derive --hook",
		}

		if err := setup.Run(cfg); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Setup complete — continuity loop wired.\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  .mcp.json          → http://localhost:%d/mcp\n", proxyPort)
		fmt.Fprintf(cmd.OutOrStdout(), "  UserPromptSubmit   → %s gate\n", flmntCmd)
		fmt.Fprintf(cmd.OutOrStdout(), "  SessionStart       → %s brief         (inject project memory)\n", flmntCmd)
		fmt.Fprintf(cmd.OutOrStdout(), "  Stop               → %s derive --hook (capture the session)\n", flmntCmd)
		return nil
	},
}

func resolveGateCmd() (string, error) {
	path, err := exec.LookPath("flmnt")
	if err != nil {
		return "flmnt", nil // fall back to bare name; PATH may differ at hook runtime
	}
	return path, nil
}

// resolveProject picks the flmnt project for derive/brief: the --project flag, else the repo's
// project_id (written by `flmnt setup --project`), else the active workspace.
func resolveProject(cmd *cobra.Command, repoDir string) string {
	if v, _ := cmd.Flags().GetString("project"); v != "" {
		return v
	}
	if pc, err := setup.LoadProjectConfig(repoDir); err == nil && pc.ProjectID != "" {
		return pc.ProjectID
	}
	return resolveActiveWorkspace(cmd)
}

func init() {
	setupCmd.Flags().String("server-url", "", "flmnt server URL (required)")
	setupCmd.Flags().String("project", "", "flmnt project id for this repo (used by derive and brief)")
	setupCmd.Flags().Int("proxy-port", 9876, "Local proxy port")
	_ = setupCmd.MarkFlagRequired("server-url")
	rootCmd.AddCommand(setupCmd)
}
