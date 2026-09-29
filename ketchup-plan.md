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

## STILL OPEN (not this change)
- A captured prompt is recorded as `decision.made`, which it is not. Ruled, planned, not built.
- `internal/derive/nominate.go` comments promise a "Phase-2 LLM pass" that does not exist; the founder
  reads that vocabulary as crossover from the benchmark. Discussion parked as `8d333082`.
