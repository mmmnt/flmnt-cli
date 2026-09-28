package cmd

import "testing"

/*
The --project flag on brief, derive, gate and every record-* command is documented as taking a

	"workspace name or id". It took only an id: resolveProject returned the flag verbatim, so a name
	went to the router as a workspace id and came back `403 workspace_not_found`. Verified against
	production: `--project sandbox` refused, `--project 503b0eda-…` accepted. Only `setup` ever
	resolved a name, through resolveWorkspaceID.

The prose was shipped in v1.10.1 alongside a test asserting the usage STRING contained "repo" — a
test over the sentence, which cannot fail when the behaviour the sentence promises is missing. This
one exercises the resolution instead.
*/
func TestProjectFlagResolvesAName(t *testing.T) {
	byName := func(arg string) (string, error) {
		if arg == "sandbox" {
			return "503b0eda-6762-448e-bce1-e340882cc9cc", nil
		}
		return "", errNoWorkspaceClient
	}

	got := resolveProjectWith("sandbox", "", byName)
	if got != "503b0eda-6762-448e-bce1-e340882cc9cc" {
		t.Errorf("a workspace NAME must resolve to its id; got %q", got)
	}
}

func TestProjectFlagPassesAnIdThrough(t *testing.T) {
	id := "503b0eda-6762-448e-bce1-e340882cc9cc"
	byName := func(arg string) (string, error) { return arg, nil }

	if got := resolveProjectWith(id, "", byName); got != id {
		t.Errorf("an id must survive resolution unchanged; got %q", got)
	}
}

/*
brief and gate run inside hooks and fail QUIET. If resolution cannot reach the server — offline, no

	credentials, an expired token — the flag value must still be used rather than the command dying.
	This is the same fallback resolveSetupProject already takes.
*/
func TestProjectFlagFallsBackToTheRawValueWhenResolutionIsUnavailable(t *testing.T) {
	unavailable := func(string) (string, error) { return "", errNoWorkspaceClient }

	if got := resolveProjectWith("sandbox", "", unavailable); got != "sandbox" {
		t.Errorf("an unreachable resolver must not lose the caller's value; got %q", got)
	}
}

/* With no flag, the repo's own pin still wins — the behaviour v1.10.0 exists to provide. */
func TestRepoConfigStillWinsWhenNoFlagIsGiven(t *testing.T) {
	byName := func(arg string) (string, error) { return arg, nil }

	if got := resolveProjectWith("", "repo-pinned-id", byName); got != "repo-pinned-id" {
		t.Errorf("the repo pin must be used when no flag is given; got %q", got)
	}
}
