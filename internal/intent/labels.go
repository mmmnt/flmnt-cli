package intent

// order is the order a message's labels are reported in: what it asks of the reader first, what it
// merely tells them last. Stable so a caller can compare two messages, and deliberately not
// alphabetical — a reader scanning a briefing wants the directions at the front.
var order = []Label{Directive, Inquiry, Assertion}

// Labels reports every intent a captured prompt carries, in `order`, without repeats.
//
// A SET, never one label: 41% of the founder's messages (82 of 202 in the real corpus) carry more than
// one. "flmnt updated. verify all commands updated/functional." asserts a fact and gives a direction;
// "has everything been tested? run a test across all api surfaces" asks and directs. A rule forced to
// pick one is wrong two times in five however good it is.
//
// Nil for a message with no sentences — no intent, rather than a default to the majority class.
func Labels(message string) []Label {
	seen := map[Label]bool{}
	for _, s := range Sentences(message) {
		seen[Of(s)] = true
	}
	var out []Label
	for _, l := range order {
		if seen[l] {
			out = append(out, l)
		}
	}
	return out
}
