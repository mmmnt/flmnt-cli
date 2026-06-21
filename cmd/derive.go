package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/derive"
	"github.com/spf13/cobra"
)

var deriveCmd = &cobra.Command{
	Use:   "derive",
	Short: "Derive structured reasoning memory from Claude Code history + git",
	Long: "Reads local Claude Code session transcripts + git history and derives decisions,\n" +
		"mistakes, and keyframes into flmnt — closing the continuity loop so each session builds\n" +
		"on the last. Default: a read-only inventory. --write imports; --hook is the Stop-hook entry.",
	RunE: runDerive,
}

func runDerive(cmd *cobra.Command, args []string) error {
	if hook, _ := cmd.Flags().GetBool("hook"); hook {
		return runHook(cmd)
	}

	repo, _ := cmd.Flags().GetString("repo")
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		repo = wd
	}

	proj, found, err := derive.ProjectForRepo(repo)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if !found {
		fmt.Fprintf(out, "No Claude Code history found for repo: %s\n\nKnown projects:\n", repo)
		if projects, derr := derive.DiscoverProjects(); derr == nil {
			for _, p := range projects {
				fmt.Fprintf(out, "  %s\n", p.Cwd)
			}
		}
		return nil
	}

	sessions, err := derive.SessionFiles(proj.Dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Repo:        %s\nProject dir: %s\nSessions:    %d\n\n", proj.Cwd, proj.Dir, len(sessions))

	if write, _ := cmd.Flags().GetBool("write"); write {
		return runWrite(cmd, proj, sessions)
	}

	outPath, _ := cmd.Flags().GetString("out")
	var jsonl *os.File
	if outPath != "" && outPath != "-" {
		jsonl, err = os.Create(outPath)
		if err != nil {
			return err
		}
		defer jsonl.Close()
	}

	w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION\tKEYFRAME\tDECISIONS\tMISTAKES\tCOMMITS\tSUBA\tBRANCH")
	var tKf, tDec, tMis, tCom, tSub int
	for _, s := range sessions {
		recs, perr := derive.ParseSession(s)
		if perr != nil {
			continue
		}
		der := derive.DeriveSession(proj.Cwd, recs)
		c := der.Counts()
		subs := len(derive.SubagentFiles(s))
		id := der.SessionID
		if len(id) > 8 {
			id = id[:8]
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%s\n",
			id, c[derive.KindKeyframe], c[derive.KindDecision], c[derive.KindMistake], c[derive.KindCommit], subs, der.Branch)
		tKf += c[derive.KindKeyframe]
		tDec += c[derive.KindDecision]
		tMis += c[derive.KindMistake]
		tCom += c[derive.KindCommit]
		tSub += subs
		if jsonl != nil {
			if b, merr := json.Marshal(der); merr == nil {
				jsonl.Write(b)
				jsonl.Write([]byte("\n"))
			}
		}
	}
	w.Flush()
	fmt.Fprintf(out, "\nCandidates: %d keyframes · %d decisions · %d mistakes · %d commits  (%d main sessions, +%d subagents)\n",
		tKf, tDec, tMis, tCom, len(sessions), tSub)
	if jsonl != nil {
		fmt.Fprintf(out, "Wrote candidate JSONL → %s\n", outPath)
	}
	return nil
}

// runWrite derives each (optionally filtered) session and imports its candidates into flmnt.
// Backfill skips cursor-processed sessions (unless --force / --session / --writer-dry-run).
func runWrite(cmd *cobra.Command, proj derive.Project, sessions []string) error {
	out := cmd.OutOrStdout()
	sessFilter, _ := cmd.Flags().GetString("session")
	force, _ := cmd.Flags().GetBool("force")

	w, err := buildWriter(cmd, proj.Cwd)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Writing to %s  project=%s  authed=%v  writer-dry-run=%v\n\n", w.Endpoint, w.ProjectID, w.AuthHeader != "", w.DryRun)

	cur := derive.LoadCursor()
	byKind := map[derive.Kind]int{}
	var totA, totS, n, curSkipped int
	for _, s := range sessions {
		if sessFilter == "" && !force && !w.DryRun && cur.Done(s) {
			curSkipped++
			continue
		}
		recs, perr := derive.ParseSession(s)
		if perr != nil {
			continue
		}
		der := derive.DeriveSession(proj.Cwd, recs)
		if sessFilter != "" && !strings.HasPrefix(der.SessionID, sessFilter) {
			continue
		}
		res, werr := w.WriteSession(der)
		if werr != nil {
			return fmt.Errorf("session %s: %w", der.SessionID, werr)
		}
		totA += res.Appended
		totS += res.Skipped
		for k, v := range res.ByKind {
			byKind[k] += v
		}
		if !w.DryRun {
			cur.Mark(s)
		}
		fmt.Fprintf(out, "  %s: appended %d, skipped %d\n", short8(der.SessionID), res.Appended, res.Skipped)
		n++
	}
	if !w.DryRun {
		_ = cur.Save()
	}
	fmt.Fprintf(out, "\nDone: %d sessions written · %d cursor-skipped · %d appended · %d skipped (idempotent)\nBy kind: %v\n",
		n, curSkipped, totA, totS, byKind)
	return nil
}

// runHook is the Stop-hook entry: reads the hook JSON from stdin (transcript_path + cwd), derives
// that one session, and imports it. Fails QUIET — a hook must never block or noise the session.
func runHook(cmd *cobra.Command) error {
	var payload struct {
		SessionID      string `json:"session_id"`
		TranscriptPath string `json:"transcript_path"`
		Cwd            string `json:"cwd"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&payload); err != nil || payload.TranscriptPath == "" {
		return nil
	}
	recs, err := derive.ParseSession(payload.TranscriptPath)
	if err != nil || len(recs) == 0 {
		return nil
	}
	w, err := buildWriter(cmd, payload.Cwd)
	if err != nil {
		return nil // not configured (no login/project) — stay quiet
	}
	der := derive.DeriveSession(payload.Cwd, recs)
	_, _ = w.WriteSession(der) // never surface write errors from a hook
	return nil
}

// buildWriter resolves the write endpoint (like `sync`: --server-url / QUORUM_SERVER_URL / login
// config; prod by default, localhost for devs), best-effort Bearer, and project (active workspace
// unless --project). Core is never targeted directly — writes go through the public /sync/import route.
func buildWriter(cmd *cobra.Command, repoDir string) (*derive.Writer, error) {
	out := cmd.OutOrStdout()
	writerDry, _ := cmd.Flags().GetBool("writer-dry-run")

	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return nil, fmt.Errorf("no server URL: run `flmnt login`, set QUORUM_SERVER_URL, or pass --server-url (e.g. http://localhost:3000 for a local stack)")
	}
	project := resolveProject(cmd, repoDir)
	if project == "" {
		return nil, fmt.Errorf("no project: pass --project, set project_id in .quorum.json (`flmnt setup --project`), or select an active workspace")
	}
	return &derive.Writer{
		Endpoint:   serverURL,
		ProjectID:  project,
		AuthHeader: bestEffortBearer(cmd, serverURL),
		DryRun:     writerDry,
		Log:        func(s string) { fmt.Fprintln(out, s) },
	}, nil
}

// bestEffortBearer returns a fresh "Bearer <token>" if logged in to serverURL, else "" (local stack
// needs no auth). Mirrors how `sync`/`mcp auth-header` mint the header.
func bestEffortBearer(cmd *cobra.Command, serverURL string) string {
	tokens, err := authHeaderLoadToken(serverURL)
	if err != nil {
		return ""
	}
	tokenURL, clientID, err := resolveOAuthEndpoint(cmd, serverURL)
	if err != nil {
		return ""
	}
	fresh, refreshed, err := auth.EnsureFreshAccessToken(tokens, tokenURL, clientID, accessTokenRefreshThreshold, time.Now())
	if err != nil {
		return ""
	}
	if refreshed {
		_ = authHeaderStoreToken(serverURL, fresh)
	}
	return "Bearer " + fresh.AccessToken
}

func short8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func init() {
	deriveCmd.Flags().Bool("dry-run", true, "Inventory only — read-only, no writes (default mode)")
	deriveCmd.Flags().String("repo", "", "Repo path to derive from (default: current directory)")
	deriveCmd.Flags().String("out", "", "Write candidate JSONL to this path (one SessionDerivation per line)")
	deriveCmd.Flags().Bool("write", false, "Write derived candidates to flmnt via the /sync/import route (Phase 3)")
	deriveCmd.Flags().String("server-url", "", "flmnt server URL (default: login config / QUORUM_SERVER_URL; pass http://localhost:3000 for a local stack)")
	deriveCmd.Flags().String("project", "", "flmnt project id to write into (default: active workspace)")
	deriveCmd.Flags().String("session", "", "Restrict to one main session (id prefix)")
	deriveCmd.Flags().Bool("writer-dry-run", false, "With --write: print the import payload instead of sending it")
	deriveCmd.Flags().Bool("hook", false, "Stop-hook mode: read the hook JSON from stdin and derive+import that one session (fails quiet)")
	deriveCmd.Flags().Bool("force", false, "With --write backfill: re-derive sessions even if the cursor marks them processed")
	rootCmd.AddCommand(deriveCmd)
}
