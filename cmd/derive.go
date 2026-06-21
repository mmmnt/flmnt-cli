package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

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

func init() {
	deriveCmd.Flags().Bool("dry-run", true, "Inventory only — read-only, no writes (currently the only mode)")
	deriveCmd.Flags().String("repo", "", "Repo path to derive from (default: current directory)")
	deriveCmd.Flags().String("out", "", "Write candidate JSONL to this path (one SessionDerivation per line)")
	rootCmd.AddCommand(deriveCmd)
}
