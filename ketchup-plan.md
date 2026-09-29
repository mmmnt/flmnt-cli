# ketchup-plan — deterministic intent classification over captured prompts

Design: flmnt plan `a244bf6d`. Rulings `cc6a9636` (deterministic or it does not happen) and
`ae838cfa` (design it now; ack list dropped).

Consumer: `flmnt brief`. Nothing read a prompt label before this, and CLAUDE.md forbids a mechanism
nothing can invoke — so the consumer lands in the same plan, not "later".

Package `internal/intent`. Read-time only: the label is computed from stored text and never written
into an entry. The log is append-only and burst 3/4 exist because version one of this rule was wrong
on real sentences; a stamped label would be permanently wrong wherever the rule improves.

## TODO
- [ ] Burst 1: Sentences splits on terminal punctuation and numbered-list markers, dropping bare markers [depends: none]
- [ ] Burst 2: a sentence ending in "?" is an inquiry [depends: 1]
- [ ] Burst 3: an imperative opener beats an interrogative one, so "do it" is a directive [depends: 1]
- [ ] Burst 4: a subordinate opener without a terminal "?" is an assertion [depends: 2]
- [ ] Burst 5: anything else is an assertion [depends: 1]
- [ ] Burst 6: Labels reports the SET of a message's sentence labels [depends: 2,3,4,5]
- [ ] Burst 7: brief surfaces outstanding directives and inquiries from prompt.captured [depends: 6]
- [ ] Burst 8: FallThroughOpeners reports the openers the lexicon missed, by frequency [depends: 5]

## Why each burst exists — all four numbers measured on the real 202-prompt corpus

- 41% of messages (82/202) carry MORE than one intent, so burst 6 returns a SET. A single label is
  wrong two times in five however good the rule is.
- Per sentence: assertion 283 (59%), directive 119 (25%), inquiry 78 (16%). Assertion is the biggest
  class and is absent from the directive-vs-discovery framing entirely.
- Burst 3 exists because `do` is both interrogative and imperative. Four corpus sentences are decided
  by ordering alone and ALL FOUR are directives — `do it`, `do both.`, `do it when CI on main passes`,
  `do not work in main`. `do it` is how this work was authorised; interrogative-first calls it a
  question.
- Burst 4 exists because 13 of 78 inquiries are subordinate clauses, not questions: "when you get
  ready to push changes to billing, there's no CI hooked up".
- Burst 1 drops bare markers because the numbered-list split promotes 20 of them ("2.", "3.") to
  sentences.
- Burst 8 exists because the imperative lexicon already leaked: "validate nothing is left over for
  staging in AWS" was missed because `validate` was not in the list. An enumerated list that cannot
  report its own gaps is the "gates sweep, never enumerate" failure mode; one that can is not.

The corpus is the founder's own transcripts and is NOT committed. Burst 8 is how the lexicon is
checked against real data without the data entering the repo.

## DONE
