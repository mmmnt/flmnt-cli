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

/*
Length is not significance, and a byte count was the only thing standing between a direction and the
record. "flmnt updated. test latest updates." is 35 bytes and is unambiguously a direction; it was
dropped for being under 120 while 236-character musings were captured as direction-setting. Ruling
cc6a9636: classification is deterministic, and where no deterministic rule has been settled the
record takes everything rather than guessing.
*/
func TestAShortDirectionIsCaptured(t *testing.T) {
	recs := []Record{{Type: "user", UUID: "u1", SessionID: "s1", Message: userMsg("flmnt updated. test latest updates.")}}

	d := NominateSession("/repo", recs)

	if got := d.Counts()[KindPrompt]; got != 1 {
		t.Errorf("prompts=%d want 1; a direction is not measured in bytes", got)
	}
}

/*
isDirective returned TRUE by default and only refused a message ending in "?" or opening with one of
sixteen question words. Measured over 72 real captures, 16 contained a "?" and were still titled
"Direction-setting message" — including "what's the login url? ... give me the path and I'll test",
which the opening-word list misses because of the apostrophe. A title is a claim, and that one was
being made on a coin flip. The record now says what it knows: a user said this.
*/
func TestAQuestionIsCapturedAndNotCalledADirection(t *testing.T) {
	recs := []Record{{Type: "user", UUID: "u1", SessionID: "s1", Message: userMsg("what's the login url?")}}

	d := NominateSession("/repo", recs)

	if got := d.Counts()[KindPrompt]; got != 1 {
		t.Fatalf("prompts=%d want 1; a question is still something the human said", got)
	}
	for _, c := range d.Candidates {
		if c.Kind == KindPrompt && c.Title != "User message" {
			t.Errorf("title=%q; a captured prompt must not claim to be direction-setting", c.Title)
		}
	}
}
