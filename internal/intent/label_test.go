package intent

import "testing"

func TestATerminalQuestionMarkIsAnInquiry(t *testing.T) {
	if got := Of("status?"); got != Inquiry {
		t.Errorf("Of(%q) = %q, want %q", "status?", got, Inquiry)
	}
}

/*
`do` is both an interrogative opener and an imperative one, so ordering alone decides it — and every
sentence in the real corpus that the ordering decides is a directive: "do it", "do both.", "do it when
CI on main passes", "do not work in main, create a feature branch". `do it` is how the supersession and
capture work was authorised. Interrogative-first files that as a question, which is why the imperative
test runs first and why this is a test rather than a comment.
*/
func TestAnImperativeOpenerBeatsAnInterrogativeOne(t *testing.T) {
	for _, s := range []string{"do it", "do both.", "do it when CI on main passes"} {
		if got := Of(s); got != Directive {
			t.Errorf("Of(%q) = %q, want %q", s, got, Directive)
		}
	}
}

func TestAnInterrogativeOpenerWithoutAQuestionMarkIsStillAnInquiry(t *testing.T) {
	if got := Of("whats your feature branch again"); got != Inquiry {
		t.Errorf("Of(%q) = %q, want %q", "whats your feature branch again", got, Inquiry)
	}
}

/*
Thirteen of the 78 sentences this rule called inquiries in the real corpus are subordinate clauses, not
questions — "when you get ready to push changes to billing, there's no CI hooked up", "If they do, you
go current immediately", "When you build page-by-page, make sure to take the page as the source of
truth". A word that opens a question can equally open a condition, and only the question mark tells
them apart.
*/
func TestASubordinateOpenerWithoutAQuestionMarkIsAnAssertion(t *testing.T) {
	for _, s := range []string{
		"when you get ready to push changes to billing, there's no CI hooked up.",
		"If they do, you go current immediately.",
	} {
		if got := Of(s); got != Assertion {
			t.Errorf("Of(%q) = %q, want %q", s, got, Assertion)
		}
	}
}

func TestASubordinateOpenerWithAQuestionMarkIsStillAnInquiry(t *testing.T) {
	if got := Of("when did that land?"); got != Inquiry {
		t.Errorf("Of(%q) = %q, want %q", "when did that land?", got, Inquiry)
	}
}
