package cmd

import (
	"fmt"
	"io"

	"github.com/mmmnt/flmnt-cli/internal/intent"
)

// intentReportOpeners caps the gap list: the tail is a long thin distribution of ordinary sentence
// openers, and a report nobody finishes reading reports nothing.
const intentReportOpeners = 15

// writeIntentReport prints how the captured prompts classify, and which openers the imperative lexicon
// did not recognise.
//
// It rules on nothing. Assertion is the majority class — 59% of real sentences — so most fall-throughs
// are genuine assertions, and no rule here can tell those from a verb the lexicon is missing. A human
// reading the list can, which is the point: an enumerated lexicon that reports its own gaps stays
// honest, and one that cannot silently stops covering what it is for.
func writeIntentReport(w io.Writer, prompts []string) {
	counts := map[intent.Label]int{}
	var sentences int
	for _, p := range prompts {
		for _, s := range intent.Sentences(p) {
			counts[intent.Of(s)]++
			sentences++
		}
	}
	fmt.Fprintf(w, "\nIntent over %d captured prompts (%d sentences):\n", len(prompts), sentences)
	for _, l := range []intent.Label{intent.Directive, intent.Inquiry, intent.Assertion} {
		fmt.Fprintf(w, "  %-10s %4d\n", l, counts[l])
	}

	gaps := intent.FallThroughOpeners(prompts)
	if len(gaps) == 0 {
		return
	}
	fmt.Fprintf(w, "\nTop unrecognised openers — a verb here is a gap in the imperative lexicon:\n")
	for i, g := range gaps {
		if i >= intentReportOpeners {
			break
		}
		fmt.Fprintf(w, "  %-14s %4d\n", g.Word, g.Count)
	}
}
