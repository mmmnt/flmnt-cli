package cmd

import "strings"
import "testing"

/*
whoami reported the machine-wide ACTIVE workspace and nothing else, which is precisely the setting

	that does NOT decide where a configured repo records. Standing in a repo pinned to quorum, with
	howie active, it answered "howie" — the exact confusion the 1.10.x line exists to remove, printed
	by the command whose whole job is to say where you are.
*/
func TestWhoamiNamesTheRepoPinWhenThereIsOne(t *testing.T) {
	line := whoamiLine("mike@flmnt.ai", "howie", "bc674142-782e-4fc3-aa79-585d12ff2d96", "quorum")

	if !strings.Contains(line, "quorum") {
		t.Errorf("the repo's own workspace must be named; got %q", line)
	}
	if !strings.Contains(line, "this repo") {
		t.Errorf("it must be clear WHY that workspace applies; got %q", line)
	}
}

/* With a pin present, the active workspace is not what governs — saying it unqualified misleads. */
func TestWhoamiDoesNotPresentTheActiveWorkspaceAsGoverningAPinnedRepo(t *testing.T) {
	line := whoamiLine("mike@flmnt.ai", "howie", "bc674142-782e-4fc3-aa79-585d12ff2d96", "quorum")

	if strings.Contains(line, "active workspace: howie") {
		t.Errorf("the active workspace must not be presented as the one in force; got %q", line)
	}
}

/* Outside a configured repo the active workspace IS what governs, and is reported as before. */
func TestWhoamiFallsBackToTheActiveWorkspace(t *testing.T) {
	line := whoamiLine("mike@flmnt.ai", "howie", "", "")

	if !strings.Contains(line, "active workspace: howie") {
		t.Errorf("with no repo pin the active workspace governs; got %q", line)
	}
}

func TestWhoamiSaysWhenThereIsNoWorkspaceAtAll(t *testing.T) {
	line := whoamiLine("mike@flmnt.ai", "", "", "")

	if !strings.Contains(line, "no active workspace") {
		t.Errorf("got %q", line)
	}
}

/*
The repo pin was printed as the raw id, so whoami answered with a UUID inside a configured repo and

	with a name outside one. setup now persists the name; whoami says it.
*/
func TestWhoamiNamesThePinnedWorkspaceRatherThanItsID(t *testing.T) {
	line := whoamiLine("mike@flmnt.ai", "howie", "bc674142-782e-4fc3-aa79-585d12ff2d96", "quorum")

	if !strings.Contains(line, "quorum") {
		t.Errorf("the workspace's name must be what a reader sees; got %q", line)
	}
	if strings.Contains(line, "bc674142") {
		t.Errorf("a UUID is not an answer to \"where am I\"; got %q", line)
	}
}

/* A repo set up before the name was persisted has no name to print — the id is still the truth. */
func TestWhoamiFallsBackToThePinnedIDWhenNoNameWasRecorded(t *testing.T) {
	line := whoamiLine("mike@flmnt.ai", "howie", "bc674142-782e-4fc3-aa79-585d12ff2d96", "")

	if !strings.Contains(line, "bc674142-782e-4fc3-aa79-585d12ff2d96") {
		t.Errorf("with no recorded name the id must still be shown; got %q", line)
	}
}
