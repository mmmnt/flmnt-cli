package derive

import "testing"

/*
A captured prompt is not a decision.

The hook records every substantive user message into the domain stream, which the founder ruled stays
— the record of what a human directed is worth keeping, and in a regulated setting it is evidence:
replayed in order, attributed to an actor, it reconstructs the path taken to a decision. But filing it
as `decision.made` put a founder's typing on the same footing as a founder's ruling, inflated every
decision count the product shows, and — until 03b643fa — held every agent behind in "Who's current".

`decision.made` now means a decision somebody recorded deliberately. Ruling ad840995.
*/
func TestACapturedPromptIsNotADecision(t *testing.T) {
	if got := entryType(KindPrompt); got != "prompt.captured" {
		t.Errorf("a captured prompt must carry its own type; got %q", got)
	}
}

/* The types derive DOES author on purpose are unchanged. */
func TestTheOtherDerivedTypesAreUnchanged(t *testing.T) {
	for kind, want := range map[Kind]string{
		KindKeyframe: "session.recap",
		KindMistake:  "decision.mistake",
		KindCommit:   "commit.recorded",
	} {
		if got := entryType(kind); got != want {
			t.Errorf("entryType(%q) = %q, want %q", kind, got, want)
		}
	}
}
