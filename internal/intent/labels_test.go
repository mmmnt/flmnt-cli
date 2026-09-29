package intent

import (
	"reflect"
	"testing"
)

/*
41% of the founder's messages (82 of 202) carry more than one intent, so a message has a SET and never
a single label. Any rule forced to pick one is wrong two times in five however good it is.
*/
func TestAMessageCarriesTheSetOfItsSentenceLabels(t *testing.T) {
	for _, tc := range []struct {
		message string
		want    []Label
	}{
		{"flmnt updated. verify all commands updated/functional.", []Label{Directive, Assertion}},
		{"has everything been tested? run a test across all api surfaces.", []Label{Directive, Inquiry}},
		{"leave them. what's next?", []Label{Directive, Inquiry}},
		{"installed", []Label{Assertion}},
		{"status?", []Label{Inquiry}},
	} {
		if got := Labels(tc.message); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Labels(%q) = %v, want %v", tc.message, got, tc.want)
		}
	}
}

// A message that says nothing claims no intent, rather than defaulting to the majority class.
func TestAMessageWithNoSentencesCarriesNoLabels(t *testing.T) {
	if got := Labels("  2. "); got != nil {
		t.Errorf("Labels of bare markers = %v, want nil", got)
	}
}
