package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/corpus"
	"github.com/spf13/cobra"
)

var corpusCmd = &cobra.Command{
	Use:   "corpus",
	Short: "Render a workspace's doctrine stream to markdown",
	Long: "Writes one markdown document per DOC-NODE entry in the workspace's domain stream, plus\n" +
		"rulings.md for doctrine that reaches no DOC-NODE. Supersessions are collapsed: the ruling\n" +
		"that stands today takes the position of the one it replaced, and the replaced text is kept\n" +
		"in a Superseded appendix. The output is GENERATED — edit the stream, never the markdown.",
	RunE: runCorpus,
}

func runCorpus(cmd *cobra.Command, args []string) error {
	err := renderCorpus(cmd)
	// A SessionStart refresh runs before anybody has typed anything. Offline, not logged in, or aimed
	// at a workspace with no DOC-NODE, it leaves the files alone and says nothing — the same contract
	// `brief` and `gate` keep in hooks. Run by hand it reports, because then somebody is listening.
	if hook, _ := cmd.Flags().GetBool("hook"); hook {
		return nil
	}
	return err
}

func renderCorpus(cmd *cobra.Command) error {
	serverURL := resolveRemoteServerURL(cmd)
	if serverURL == "" {
		return fmt.Errorf("--server-url or QUORUM_SERVER_URL is required")
	}
	cwd, _ := os.Getwd()
	project := resolveProject(cmd, cwd)
	if project == "" {
		return errNoWorkspaceClient
	}
	gql, err := graphQLClientFor(cmd, serverURL)
	if err != nil {
		return err
	}

	streamID := project + "::domain"
	entries, err := corpus.Fetch(gql, streamID)
	if err != nil {
		return err
	}
	report := corpus.Render(streamID, entries)
	if len(report.Files) == 0 {
		return fmt.Errorf("%s holds no DOC-NODE entry, so there is no document to render", streamID)
	}

	out, _ := cmd.Flags().GetString("out")
	if _, err := corpus.Write(out, report); err != nil {
		return err
	}
	stray, err := corpus.Stray(out, report)
	if err != nil {
		return err
	}
	if hook, _ := cmd.Flags().GetBool("hook"); !hook {
		fmt.Fprint(cmd.OutOrStdout(), corpusSummary(out, report, stray))
	}
	return nil
}

// corpusSummary reports what the render produced AND what it did not: the entry types left out, and
// any markdown sitting in the directory that this render did not generate. A generator that prints
// only its own output is how a stale hand-authored document survives beside a generated one.
func corpusSummary(dir string, r corpus.Report, stray []string) string {
	var b strings.Builder
	for _, f := range r.Files {
		fmt.Fprintf(&b, "%s  %d sections\n", filepath.Join(dir, f.Name), f.Sections)
	}
	if len(r.Unrendered) > 0 {
		kinds := make([]string, 0, len(r.Unrendered))
		for kind := range r.Unrendered {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		parts := make([]string, 0, len(kinds))
		for _, kind := range kinds {
			parts = append(parts, fmt.Sprintf("%s %d", kind, r.Unrendered[kind]))
		}
		fmt.Fprintf(&b, "not doctrine, rendered nowhere: %s\n", strings.Join(parts, ", "))
	}
	if len(stray) > 0 {
		fmt.Fprintf(&b, "in %s but not generated — stale unless you keep them deliberately: %s\n", dir, strings.Join(stray, ", "))
	}
	return b.String()
}

func init() {
	corpusCmd.Flags().String("server-url", "", "flmnt server URL (default: login config / QUORUM_SERVER_URL)")
	corpusCmd.Flags().String("project", "", "workspace name or id whose doctrine stream to render (default: this repo's own setting)")
	corpusCmd.Flags().String("out", "training", "directory to write the documents into")
	corpusCmd.Flags().Bool("hook", false, "SessionStart mode: refresh silently and never fail a session start")
	rootCmd.AddCommand(corpusCmd)
}
