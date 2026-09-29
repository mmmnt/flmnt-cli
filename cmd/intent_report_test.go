package cmd

import (
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/intent"
)

/*
The report has to be reachable or the lexicon's gaps stay invisible, which is the whole reason it
exists. It states counts and openers and rules on nothing: assertion is the majority class, so most
fall-throughs are genuine — a human reads the list and spots a verb sitting where it does not belong.
*/
func TestIntentReportShowsTheLabelSplitAndTheLexiconsGaps(t *testing.T) {
	var b strings.Builder

	writeIntentReport(&b, []string{
		"cut the release",
		"was that fix pushed?",
		"provision the secret",
	})

	out := b.String()
	for _, want := range []string{
		string(intent.Directive), string(intent.Inquiry), string(intent.Assertion),
		"provision",
		"unrecognised openers",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cut") {
		t.Errorf("a verb already in the lexicon is not a gap and must not be listed:\n%s", out)
	}
}
