package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

/*
A repo set up without a project wrote `.quorum.json` carrying only a server_url, so

	resolveProject found no project_id and fell through to the machine-wide ACTIVE workspace. Every
	such repo then recorded its sessions wherever the CLI happened to point — quorum's sessions
	landed in howie for a day. A repo must name its workspace, and naming it must not require
	knowing a UUID.
*/
func TestSetupRequiresAProjectSoARepoCannotInheritTheActiveWorkspace(t *testing.T) {
	ann := setupCmd.Flags().Lookup("project").Annotations[cobra.BashCompOneRequiredFlag]
	if len(ann) == 0 || ann[0] != "true" {
		t.Error("expected --project to be required: without it a repo silently inherits the active workspace")
	}
}

func TestSetupProjectFlagTakesANameNotOnlyAnID(t *testing.T) {
	usage := setupCmd.Flags().Lookup("project").Usage
	if !strings.Contains(usage, "name") {
		t.Errorf("expected --project to document a workspace NAME; a UUID is not something a person knows. got %q", usage)
	}
}

/*
Every command that resolves a project says where the default comes from, and the answer had gone

	stale: "default: active workspace" omits the repo's own .quorum.json, which is now the primary
	source and the whole point of --project. A reader following that text would believe the
	machine-wide setting decides — the exact belief that let one project's sessions be written into
	another project's stream for a day.
*/
func TestEveryProjectFlagDocumentsTheRepoConfigAsTheDefault(t *testing.T) {
	for _, c := range []*cobra.Command{briefCmd, deriveCmd, recordMetricCmd} {
		f := c.Flags().Lookup("project")
		if f == nil {
			t.Fatalf("%s: expected a --project flag", c.Name())
		}
		if strings.Contains(f.Usage, "default: active workspace") {
			t.Errorf("%s: usage still names the active workspace as THE default: %q", c.Name(), f.Usage)
		}
		if !strings.Contains(f.Usage, "repo") {
			t.Errorf("%s: usage must say the repo's own setting is used first, got %q", c.Name(), f.Usage)
		}
	}
}
