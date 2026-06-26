package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/record"
	"github.com/spf13/cobra"
)

// newRecordClient resolves the endpoint, project (= workspace), and best-effort bearer the same way
// brief/derive do, and returns a ready record.Client plus the resolved project id.
func newRecordClient(cmd *cobra.Command) (record.Client, string, error) {
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return record.Client{}, "", fmt.Errorf("no server URL: run `flmnt login`, set QUORUM_SERVER_URL, or pass --server-url")
	}
	cwd, _ := os.Getwd()
	project := resolveProject(cmd, cwd)
	if project == "" {
		// Fall back to the workspace embedded in the server_url (?workspace=<id>), as written by login.
		project = record.WorkspaceFromURL(serverURL)
	}
	if project == "" {
		return record.Client{}, "", fmt.Errorf("no project: pass --project, set project_id in .quorum.json, or select an active workspace")
	}
	return record.Client{
		Endpoint:   serverURL,
		ProjectID:  project,
		AuthHeader: bestEffortBearer(cmd, serverURL),
	}, project, nil
}

func recordFlags(c *cobra.Command) {
	c.Flags().String("server-url", "", "flmnt server URL (default: login config / QUORUM_SERVER_URL)")
	c.Flags().String("project", "", "flmnt project id (default: active workspace)")
}

// parseLabels turns "k=v,k2=v2" into a map.
func parseLabels(s string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		if k, v, ok := strings.Cut(pair, "="); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

// contentArg returns the first non-empty of the flag value, the positional args, or piped stdin.
func contentArg(cmd *cobra.Command, flag string, args []string) string {
	if v, _ := cmd.Flags().GetString(flag); strings.TrimSpace(v) != "" {
		return v
	}
	if len(args) > 0 {
		return strings.TrimSpace(strings.Join(args, " "))
	}
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		if b, _ := io.ReadAll(io.LimitReader(os.Stdin, 1<<20)); len(b) > 0 {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

/* ---------- record-metric ---------- */
var recordMetricCmd = &cobra.Command{
	Use:   "record-metric",
	Short: "Record an operational metric to the project's metrics stream",
	Long: "Writes a metric (POST /streams/{project}::metrics/metrics) — the deterministic counterpart\n" +
		"to the record_metric MCP tool. With --hook it reads a PostToolUse JSON payload from stdin and\n" +
		"emits a CI/throughput metric for the command that just ran (used by the PostToolUse(Bash) hook).",
	RunE: func(cmd *cobra.Command, args []string) error {
		hook, _ := cmd.Flags().GetBool("hook")
		if hook {
			runMetricHook(cmd)
			return nil // a hook must never fail the tool that triggered it
		}
		c, project, err := newRecordClient(cmd)
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("--name is required")
		}
		value, _ := cmd.Flags().GetFloat64("value")
		labels := parseLabels(mustString(cmd, "labels"))
		stream, _ := cmd.Flags().GetString("stream")
		if stream == "" {
			stream = record.MetricsStream(project)
		}
		return c.Metric(stream, name, value, labels)
	},
}

var ciCommandRe = regexp.MustCompile(`(?i)\b(turbo (?:build|test|lint|typecheck)|vitest|pytest|jest|playwright|cdk deploy|serverless deploy|sls deploy|[a-z_]+:deploy|tsc|eslint|golangci-lint|go test)\b`)

type hookPayload struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// classifyHookMetric parses a PostToolUse(Bash) payload and returns the metric labels for a CI-shaped
// command, or ok=false when the payload isn't parseable or isn't a CI command. Pure (no network) so it
// is unit-testable; runMetricHook wraps it with the actual write.
func classifyHookMetric(raw []byte) (map[string]string, bool) {
	var p hookPayload
	if json.Unmarshal(raw, &p) != nil {
		return nil, false
	}
	command := strings.Join(strings.Fields(p.ToolInput.Command), " ")
	m := ciCommandRe.FindString(command)
	if m == "" {
		return nil, false // not a CI-shaped command — nothing to record
	}
	category := strings.ToLower(strings.Fields(m)[0])
	if strings.Contains(strings.ToLower(m), "deploy") {
		category = "deploy"
	}
	outcome := "ok"
	if resp := strings.ToLower(string(p.ToolResponse)); strings.Contains(resp, "error") || strings.Contains(resp, "fail") {
		outcome = "fail"
	}
	return map[string]string{"category": category, "outcome": outcome, "command": truncate(command, 160), "tool": "Bash"}, true
}

// runMetricHook parses a PostToolUse(Bash) payload from stdin and records a best-effort CI metric.
// Silent + non-failing by design.
func runMetricHook(cmd *cobra.Command) {
	raw, _ := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	labels, ok := classifyHookMetric(raw)
	if !ok {
		return
	}
	c, project, err := newRecordClient(cmd)
	if err != nil {
		return
	}
	_ = c.Metric(record.MetricsStream(project), "ci.run", 1, labels)
}

/* ---------- record-plan ---------- */
var recordPlanCmd = &cobra.Command{
	Use:   "record-plan",
	Short: "Record a multi-step plan snapshot to the project's plan stream",
	Long:  "Writes a plan (POST /streams/{project}::plan/plans) — the deterministic counterpart to record_plan.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, project, err := newRecordClient(cmd)
		if err != nil {
			return err
		}
		content := contentArg(cmd, "content", args)
		if content == "" {
			return fmt.Errorf("plan content required (--content, an argument, or piped stdin)")
		}
		stream, _ := cmd.Flags().GetString("stream")
		if stream == "" {
			stream = record.PlanStream(project)
		}
		return c.Plan(stream, content)
	},
}

/* ---------- record-supersession ---------- */
var recordSupersessionCmd = &cobra.Command{
	Use:   "record-supersession",
	Short: "Record a decision that replaces a prior one (creates a SUPERSEDED_BY edge)",
	Long:  "Writes a superseding decision (POST /streams/{id}/supersessions) — the deterministic counterpart to record_supersession.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, project, err := newRecordClient(cmd)
		if err != nil {
			return err
		}
		content := contentArg(cmd, "content", args)
		supersedes, _ := cmd.Flags().GetString("supersedes")
		if content == "" || strings.TrimSpace(supersedes) == "" {
			return fmt.Errorf("--content and --supersedes (prior entry id) are both required")
		}
		stream, _ := cmd.Flags().GetString("stream")
		if stream == "" {
			stream = record.DomainStream(project)
		}
		return c.Supersession(stream, content, supersedes)
	},
}

/* ---------- record-attestation ---------- */
var recordAttestationCmd = &cobra.Command{
	Use:   "record-attestation",
	Short: "Record a ContextAttestation metric (flmnt context changed the outcome)",
	Long:  "Emits a ContextAttestation metric (value=1, labels {kind, note}) to {project}::metrics — the deterministic counterpart to record_attestation.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, project, err := newRecordClient(cmd)
		if err != nil {
			return err
		}
		kind, _ := cmd.Flags().GetString("kind")
		if strings.TrimSpace(kind) == "" {
			return fmt.Errorf("--kind is required")
		}
		note, _ := cmd.Flags().GetString("note")
		return c.Attestation(project, kind, note)
	},
}

func mustString(cmd *cobra.Command, name string) string { v, _ := cmd.Flags().GetString(name); return v }

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func init() {
	for _, c := range []*cobra.Command{recordMetricCmd, recordPlanCmd, recordSupersessionCmd, recordAttestationCmd} {
		recordFlags(c)
	}
	recordMetricCmd.Flags().Bool("hook", false, "read a PostToolUse JSON payload from stdin and emit a CI metric")
	recordMetricCmd.Flags().String("name", "", "metric name")
	recordMetricCmd.Flags().Float64("value", 1, "metric value")
	recordMetricCmd.Flags().String("labels", "", "comma-separated key=value labels")
	recordMetricCmd.Flags().String("stream", "", "target stream id (default: {project}::metrics)")

	recordPlanCmd.Flags().String("content", "", "plan content (or pass as args / piped stdin)")
	recordPlanCmd.Flags().String("stream", "", "target stream id (default: {project}::plan)")

	recordSupersessionCmd.Flags().String("content", "", "the superseding decision content")
	recordSupersessionCmd.Flags().String("supersedes", "", "entry id of the decision being replaced (required)")
	recordSupersessionCmd.Flags().String("stream", "", "stream for the new decision (default: {project}::domain)")

	recordAttestationCmd.Flags().String("kind", "", "attestation kind (required)")
	recordAttestationCmd.Flags().String("note", "", "what the context was + how it changed the outcome")

	rootCmd.AddCommand(recordMetricCmd, recordPlanCmd, recordSupersessionCmd, recordAttestationCmd)
}
