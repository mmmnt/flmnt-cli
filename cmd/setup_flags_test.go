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
