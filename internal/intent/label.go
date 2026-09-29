package intent

import "strings"

// Label names what one sentence does. Three, because the real corpus has three: of 480 sentences the
// founder actually wrote, 283 were assertions, 119 directives and 78 inquiries. A directive/inquiry
// binary has nowhere to put the majority class.
type Label string

const (
	// Directive: do this.
	Directive Label = "directive"
	// Inquiry: tell me this.
	Inquiry Label = "inquiry"
	// Assertion: this is so. The biggest class, and frequently the cause of the next directive.
	Assertion Label = "assertion"
)

// Of classifies one sentence. Deterministic and order-dependent; the order IS the rule.
func Of(s string) Label {
	if strings.HasSuffix(strings.TrimSpace(s), "?") {
		return Inquiry
	}
	return Assertion
}
