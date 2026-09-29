package intent

import "testing"

/*
The imperative lexicon is an enumerated list, and an enumerated list silently stops covering what it is
for — "validate nothing is left over for staging in AWS" was a direction the first version missed
because `validate` was absent from it. (It is in the lexicon now, which is why this test needs other
words to demonstrate a gap — and is itself the evidence that gaps happen.)

This does not guess at the gap. It surfaces every opener that fell through to assertion, by frequency,
so a human reading real text can see a verb sitting where it does not belong. Enumeration that can
report its own gaps is not the failure mode the repo's "gates sweep, never enumerate" law is about;
enumeration that cannot is.
*/
func TestFallThroughOpenersRanksWhatTheLexiconMissed(t *testing.T) {
	got := FallThroughOpeners([]string{
		"provision the secret in SSM",
		"provision the grants secret",
		"seed the workspace",
		"cut the release",
		"status?",
	})

	if len(got) != 2 {
		t.Fatalf("got %v, want two openers — a verb already in the lexicon is not a gap, nor is an inquiry", got)
	}
	if got[0].Word != "provision" || got[0].Count != 2 {
		t.Errorf("got[0] = %+v, want provision x2 ranked first", got[0])
	}
	if got[1].Word != "seed" || got[1].Count != 1 {
		t.Errorf("got[1] = %+v, want seed x1", got[1])
	}
}

// Stable run to run, so the report can be diffed: frequency first, then alphabetical.
func TestFallThroughOpenersBreaksTiesAlphabetically(t *testing.T) {
	got := FallThroughOpeners([]string{"seed the workspace", "provision the secret"})

	if len(got) != 2 || got[0].Word != "provision" || got[1].Word != "seed" {
		t.Errorf("got %v, want provision before seed at equal counts", got)
	}
}
