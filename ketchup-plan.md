# ketchup-plan — derive's writes carry their author

Part of the "Who's current" work (quorum plan `cada32c2`, ruling `ad840995`).

## DONE
- [x] `internal/derive/writer.go` posts `memoryDerive` instead of `memoryImport`

Derive AUTHORS the entries it writes, but it posted them through `memoryImport` — the replication
mutation, which deliberately attributes nobody so that moving someone else's history cannot acquire an
author from whoever ran the move. The consequence was that every derived decision landed authorless,
and an authorless decision used to hold every agent behind in the dashboard's "Who's current".

Verified by re-introduction: pointing the writer back at `memoryImport` fails the test by name.
Full gate green — 13 packages, `go vet` clean, `gofmt` clean.

## BLOCKED ON A RELEASE ORDER — do not tag until quorum v1.10.12 is LIVE
`memoryDerive` only becomes reachable once the dashboard deploy recomposes `router.json` from the
memory subgraph's SDL (`docker/supergraph.production.yaml` reads
`packages/core/src/graphql/schema.graphql`). Releasing this CLI first would make every `flmnt derive`
fail on an unknown field — and `--hook` swallows write errors, so it would fail SILENTLY.
Probe production for the mutation before tagging.

## DONE
- [x] a captured prompt is written as `prompt.captured`, not `decision.made`

`KindDecision` is now `KindPrompt`, and it has exactly one producer — the user-message branch of
nomination. Derive no longer writes `decision.made` at all. The capture stays: replayed in order and
attributed, prompts reconstruct the path an actor took to a ruling, which in a regulated market is
the evidence. What it stops doing is claiming a founder's typing was a decision somebody recorded —
which inflated every decision count the product shows and moved a bar in "Who's current" that its
own author could not clear.

Read-side checked before the retype, not after: retrieval does not filter by entry type (the RLM
special-cases only `decision.mistake`), entry types are not validated on write, and the unknown-type
fallbacks are graceful (`threadKind` → `question`, `kindOf` → the type verbatim). `Correlate` is
Kind-agnostic, so replay linkage survives unchanged. The 14 non-test `decision.made` consumers in
quorum keep working; they simply stop counting prompts.

- [x] the "Phase-2 LLM pass" comments are gone

There is no Phase-2 pass. `DeriveSession` runs nomination straight into Refine, Correlate and the
writer, so every threshold in `nominate.go` IS the shipped precision, not a pre-filter for a judge
that would catch the rest. The comments said otherwise in four places and were the standing licence
for over-collection. The founder reads that vocabulary as crossover from the benchmark, where all
judging actually lives. Discussion `8d333082`.
