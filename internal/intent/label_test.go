package intent

import "testing"

func TestATerminalQuestionMarkIsAnInquiry(t *testing.T) {
	if got := Of("status?"); got != Inquiry {
		t.Errorf("Of(%q) = %q, want %q", "status?", got, Inquiry)
	}
}
