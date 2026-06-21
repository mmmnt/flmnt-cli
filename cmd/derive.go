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
	Long: "Reads local Claude Code session transcripts (and, later, git history) and derives\n" +
		"decisions, mistakes, and keyframes into flmnt — closing the continuity loop so each\n" +
		"session builds on the last.\n\n" +
		"This build implements --dry-run only: a read-only inventory of what would be derived.",
	RunE: runDerive,
}

func runDerive(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	repo, _ := cmd.Flags().GetString("repo")
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		repo = wd
	}
	if !dryRun {
		return fmt.Errorf("only --dry-run is implemented in this build")
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

// runWrite derives each (optionally filtered) session and writes its candidates to a flmnt core,
// wiring causal edges. Used for the Phase-3 local validation.
func runWrite(cmd *cobra.Command, proj derive.Project, sessions []string) error {
	out := cmd.OutOrStdout()
	project, _ := cmd.Flags().GetString("project")
	sessFilter, _ := cmd.Flags().GetString("session")
	writerDry, _ := cmd.Flags().GetBool("writer-dry-run")

	// Resolve the write endpoint the SAME way `sync` does: --server-url / QUORUM_SERVER_URL /
	// login config. Defaults to the login-configured server (prod for users); devs override to a
	// local stack. Core is never targeted directly — writes go through the public /sync/import route.
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return fmt.Errorf("no server URL: run `flmnt login`, set QUORUM_SERVER_URL, or pass --server-url (e.g. http://localhost:3000 for a local stack)")
	}
	authHeader := bestEffortBearer(cmd, serverURL)
	if project == "" {
		project = resolveActiveWorkspace(cmd)
	}
	if project == "" {
		return fmt.Errorf("no project: pass --project or select an active workspace (`flmnt workspace use`)")
	}

	w := &derive.Writer{
		Endpoint:   serverURL,
		ProjectID:  project,
		AuthHeader: authHeader,
		DryRun:     writerDry,
		Log:        func(s string) { fmt.Fprintln(out, s) },
	}
	fmt.Fprintf(out, "Writing to %s  project=%s  authed=%v  writer-dry-run=%v\n\n", serverURL, project, authHeader != "", writerDry)

	byKind := map[derive.Kind]int{}
	var totA, totS, n int
	for _, s := range sessions {
		recs, perr := derive.ParseSession(s)
		if perr != nil {
			continue
		}
		der := derive.DeriveSession(proj.Cwd, recs)
		if sessFilter != "" && !strings.HasPrefix(der.SessionID, sessFilter) {
			continue
		}
		res, err := w.WriteSession(der)
		if err != nil {
			return fmt.Errorf("session %s: %w", der.SessionID, err)
		}
		totA += res.Appended
		totS += res.Skipped
		for k, v := range res.ByKind {
			byKind[k] += v
		}
		fmt.Fprintf(out, "  %s: appended %d, skipped %d\n", short8(der.SessionID), res.Appended, res.Skipped)
		n++
	}
	fmt.Fprintf(out, "\nDone: %d sessions · %d appended · %d skipped (idempotent re-import)\nBy kind (candidates): %v\n", n, totA, totS, byKind)
	return nil
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
	deriveCmd.Flags().String("project", "", "Quorum project id to write into (default: active workspace)")
	deriveCmd.Flags().String("session", "", "Restrict to one main session (id prefix)")
	deriveCmd.Flags().Bool("writer-dry-run", false, "With --write: print the import payload instead of sending it")
	rootCmd.AddCommand(deriveCmd)
}
