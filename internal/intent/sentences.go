// Package intent classifies what a captured prompt DOES, deterministically and at read time.
//
// Deterministic is the whole point: the product claims its classification is rule-based, so no model
// judges intent here. Read-time is the consequence: a label computed from stored text is reproducible
// from (text, rule) and improves for every entry ever written when the rule improves, where a label
// stamped into an append-only log is permanently wrong wherever the rule was.
package intent

import (
	"regexp"
	"strings"
	"unicode"
)

// listMarker matches a numbered-list marker at the start of what remains ("1. ", "2.1) ").
var listMarker = regexp.MustCompile(`^\d+(?:\.\d+)*[.)](\s|$)`)

// bareMarker matches a marker with nothing after it. The split promotes these to sentences — twenty
// of them across the real corpus — and a marker states no intent, so it is dropped, not labelled.
var bareMarker = regexp.MustCompile(`^\d+(?:\.\d+)*[.)]?$`)

// Sentences splits a captured prompt into the units that carry intent.
//
// Two boundaries, not one: terminal punctuation, AND the start of a numbered-list marker. The markers
// matter because this is how directions actually arrive — "1. includes 2. is there a question here?
// 3. create the query we need" is three intents, and terminal punctuation alone reads it as one.
// RE2 has no lookaround, so the scan is explicit rather than a split pattern.
func Sentences(text string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		s := strings.TrimSpace(cur.String())
		cur.Reset()
		if s == "" || bareMarker.MatchString(s) {
			return
		}
		out = append(out, s)
	}
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		rest := string(runes[i:])
		// A marker opens a new unit and is carried into it whole. Consuming it here is what keeps its
		// own period from reading as terminal punctuation, which would strip "1." off "1. includes"
		// and leave the marker behind as a bare unit.
		if m := listMarker.FindString(rest); m != "" && (i == 0 || unicode.IsSpace(runes[i-1])) {
			flush()
			marker := strings.TrimRight(m, " \t\n")
			cur.WriteString(marker)
			i += len([]rune(marker)) - 1
			continue
		}
		cur.WriteRune(runes[i])
		if (runes[i] == '.' || runes[i] == '!' || runes[i] == '?') && i+1 < len(runes) && unicode.IsSpace(runes[i+1]) {
			flush()
		}
	}
	flush()
	return out
}
