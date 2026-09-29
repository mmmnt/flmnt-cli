package intent

import (
	"regexp"
	"strings"
)

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

// imperativeOpeners are the verbs a direction actually starts with. A lexicon is unavoidable without
// a part-of-speech tagger, and an enumerated list silently stops covering what it is for — so
// FallThroughOpeners exists to report its gaps against real text instead of letting them rot.
var imperativeOpeners = map[string]bool{
	"add": true, "annotate": true, "answer": true, "apply": true, "audit": true, "avoid": true,
	"build": true, "bump": true, "check": true, "close": true, "commit": true, "confirm": true,
	"connect": true, "continue": true, "create": true, "cut": true, "deactivate": true,
	"delete": true, "deploy": true, "design": true, "do": true, "don't": true, "dont": true,
	"drop": true, "ensure": true, "explain": true, "finish": true, "fix": true, "follow": true,
	"give": true,
	"go":   true, "guide": true, "handle": true, "hold": true, "ignore": true, "install": true,
	"keep": true, "leave": true, "let": true, "let's": true, "lets": true, "link": true,
	"list": true,
	"make": true, "measure": true, "merge": true, "move": true, "open": true, "place": true,
	"proceed": true, "pull": true, "push": true, "read": true, "record": true, "release": true,
	"remove": true, "replace": true, "report": true, "revert": true, "run": true, "send": true,
	"show": true, "skip": true, "split": true, "start": true, "stop": true, "sweep": true,
	"tag": true, "test": true, "translate": true, "undo": true, "update": true, "use": true,
	"validate": true, "verify": true, "wait": true, "work": true, "write": true,
}

// interrogativeOpeners begin an inquiry even when the writer left the question mark off, which they
// often do: "whats your feature branch again", "were those stale docs updated".
var interrogativeOpeners = map[string]bool{
	"am": true, "any": true, "anyone": true, "anything": true, "are": true, "aren't": true,
	"can": true, "could": true, "did": true, "didn't": true, "do": true, "does": true,
	"had": true, "has": true, "have": true, "how": true, "is": true, "isn't": true,
	"question": true, "should": true, "was": true, "were": true, "what": true, "what's": true,
	"whats": true, "when": true, "where": true, "which": true, "who": true, "whose": true,
	"why": true, "will": true, "would": true,
}

// subordinateOpeners open a question and a condition equally, so only a terminal question mark tells
// them apart. Without one they are asserting something: thirteen of the corpus's inquiries were
// clauses like "when you get ready to push changes to billing, there's no CI hooked up".
var subordinateOpeners = map[string]bool{
	"as": true, "if": true, "since": true, "when": true, "where": true, "while": true,
}

var leadingMarker = regexp.MustCompile(`^\d+(?:\.\d+)*[.)]\s*`)
var firstWord = regexp.MustCompile(`^[a-z'-]+`)

// Of classifies one sentence. Deterministic and ORDER-DEPENDENT; the order is the rule.
//
// The imperative test runs before the interrogative one because `do` belongs to both sets, and every
// sentence in the real corpus that the ordering decides is a direction: "do it", "do both.", "do it
// when CI on main passes", "do not work in main". Interrogative-first files those as questions.
func Of(s string) Label {
	t := strings.TrimSpace(s)
	if strings.HasSuffix(t, "?") {
		return Inquiry
	}
	word := opener(s)
	switch {
	case imperativeOpeners[word]:
		return Directive
	case subordinateOpeners[word]:
		return Assertion
	case interrogativeOpeners[word]:
		return Inquiry
	default:
		return Assertion
	}
}

// opener is a sentence's first word, lowercased, with any list marker stripped.
func opener(s string) string {
	return firstWord.FindString(leadingMarker.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), ""))
}
