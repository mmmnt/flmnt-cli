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
	Long: `Installs the flmnt automation kit: a direct flmnt entry in .mcp.json (use --proxy for the
local-proxy entry instead), the full lifecycle hook map + flmnt MCP tool permissions in
.claude/settings.local.json, the slash-command catalog in .claude/commands/, and the nudge/gate
scripts in .claude/flmnt-hooks/. Other .mcp.json servers are preserved. Idempotent — safe to re-run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString("server-url")
		proxyPort, _ := cmd.Flags().GetInt("proxy-port")
		project, _ := cmd.Flags().GetString("project")
		proxy, _ := cmd.Flags().GetBool("proxy")

		flmntCmd, err := resolveGateCmd()
		if err != nil {
			return fmt.Errorf("cannot locate flmnt binary: %w", err)
		}

		// A workspace NAME is what a person knows; the id is what the config needs. Resolve the one
		// into the other here rather than asking anybody to look up a UUID — the same resolution
		// `flmnt workspace use` already performs. An id passed straight through still resolves.
		projectID, projectName, err := resolveSetupProject(cmd, project)
		if err != nil {
			return err
		}

		cfg := setup.Config{
			ServerURL: serverURL,
			ProjectID: projectID,
			ProxyPort: proxyPort,
			Proxy:     proxy,
			GateCmd:   flmntCmd + " gate",
			BriefCmd:  flmntCmd + " brief",
			DeriveCmd: flmntCmd + " derive --hook",
		}

		if err := setup.Run(cfg); err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Setup complete — flmnt automation kit installed.\n")
		fmt.Fprintf(out, "  project                → %s (%s) — this repo records here regardless of the active workspace\n", projectName, projectID)
		if proxy {
			fmt.Fprintf(out, "  .mcp.json              → flmnt-proxy @ http://localhost:%d/mcp (run `flmnt proxy`; other servers preserved)\n", proxyPort)
		} else {
			fmt.Fprintf(out, "  .mcp.json              → flmnt (direct, OAuth on first /mcp; other servers preserved)\n")
		}
		fmt.Fprintf(out, "  hooks (settings.local) → SessionStart·UserPromptSubmit·PreToolUse·PostToolUse·PreCompact·SubagentStop·Stop·SessionEnd\n")
		fmt.Fprintf(out, "  .claude/commands/      → 13 /flmnt-* slash commands\n")
		fmt.Fprintf(out, "  .claude/flmnt-hooks/   → nudge + causal-ref-gate scripts\n")
		fmt.Fprintf(out, "  permissions            → flmnt MCP tools granted\n")
		return nil
	},
}

// resolveSetupProject turns the --project argument (a workspace NAME or an id) into the id written
// to the repo config. Falls back to treating the argument as an id when the workspace list cannot be
// reached, so setup still works offline against a known id.
func resolveSetupProject(cmd *cobra.Command, arg string) (id, name string, err error) {
	client, cerr := newWorkspaceClient(cmd)
	if cerr != nil {
		return arg, arg, nil
	}
	id, name, rerr := resolveWorkspaceID(client, arg)
	if rerr != nil {
		return "", "", rerr
	}
	return id, name, nil
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
	setupCmd.Flags().String("project", "", "workspace name or id this repo records into (used by brief, derive, gate and record) (required)")
	setupCmd.Flags().Bool("proxy", false, "wire the local-proxy entry (run `flmnt proxy`) instead of the direct OAuth entry — for CI / non-OAuth clients")
	setupCmd.Flags().Int("proxy-port", 9876, "Local proxy port (used with --proxy)")
	_ = setupCmd.MarkFlagRequired("server-url")
	// Required, because the fallback it replaces was silent: a repo with no project_id resolved to
	// the machine-wide ACTIVE workspace, so whichever repo you set up last decided where every other
	// repo's sessions were recorded. quorum's sessions were written into howie for a day that way.
	_ = setupCmd.MarkFlagRequired("project")
	rootCmd.AddCommand(setupCmd)
}
