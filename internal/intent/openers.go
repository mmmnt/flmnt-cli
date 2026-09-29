package intent

import "sort"

// Opener is one first word the lexicon did not recognise, with how often it led a sentence.
type Opener struct {
	Word  string
	Count int
}

// FallThroughOpeners ranks the openers of every sentence that fell through to Assertion.
//
// The imperative lexicon is enumerated, and an enumerated list silently stops covering what it is for:
// "validate nothing is left over for staging in AWS" was a direction the first version missed because
// `validate` was absent. This makes that visible instead of guessing at it. It decides nothing — most
// fall-throughs are genuine assertions, since assertion is the majority class — it puts the openers in
// front of a human, who can see a verb sitting where it does not belong. Enumeration that can report
// its own gaps is not the failure mode "gates sweep, never enumerate" is about; enumeration that
// cannot is.
func FallThroughOpeners(texts []string) []Opener {
	counts := map[string]int{}
	for _, t := range texts {
		for _, s := range Sentences(t) {
			if Of(s) != Assertion {
				continue
			}
			if w := opener(s); w != "" {
				counts[w]++
			}
		}
	}
	out := make([]Opener, 0, len(counts))
	for w, n := range counts {
		out = append(out, Opener{Word: w, Count: n})
	}
	// Frequency first, then alphabetical so the report is stable run to run.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Word < out[j].Word
	})
	return out
}
