# ketchup-plan — `flmnt corpus`

Founder ruling 2026-09-28: the training/*.md doctrine snapshots are GENERATED from the flmnt
stream, never hand-maintained. The renderer lives here (not in quorum) because the CLI already
owns production auth and the router GraphQL client, and because every repo carrying the kit wants
the same rendering — quorum's `training/` becomes the output of `flmnt corpus --out training`.

## Derivable structure (verified against d6dd5b65::domain, 187 entries, 2026-09-28)
- A DOC-NODE entry is `decision.made` whose content starts `DOC-NODE · <TITLE> (page N) — …`.
  Four exist. Each is one output file.
- A section entry reaches its doc-node by walking `causationId`. 102 of 187 entries do.
- A supersession is `decision.superseded`; its `causationId` IS the id it replaces. No graph query
  needed — the collapse is computable from the slice alone.
- 30 `decision.made` entries reach no doc-node (later founder rulings). They are doctrine and must
  render, not vanish → `rulings.md`.
- keyframes (50), explorations (4), plan snapshot (1), stream_created (1) are state/tentative, not
  doctrine: rendered nowhere, but COUNTED in the report so the drift is visible, never silent.

## TODO
- [ ] Burst 1: a DOC-NODE entry becomes a file named after its title [depends: none]
- [ ] Burst 2: an entry whose causation chain reaches a doc-node is a section of that file [depends: 1]
- [ ] Burst 3: sections render in stream order [depends: 2]
- [ ] Burst 4: a section's heading is its content's first sentence, its body the rest [depends: 2]
- [ ] Burst 5: every section is stamped with its entry id and timestamp [depends: 4]
- [ ] Burst 6: a superseded section renders its superseder's content in its own position [depends: 3]
- [ ] Burst 7: the replaced text survives in a Superseded appendix naming the superseder [depends: 6]
- [ ] Burst 8: a supersession chain renders only the final superseder as current [depends: 6]
- [ ] Burst 9: a doctrine decision that reaches no doc-node renders into rulings.md [depends: 2]
- [ ] Burst 10: non-doctrine entry types render nowhere and are counted [depends: 9]
- [ ] Burst 11: `flmnt corpus --out <dir>` fetches the slice and writes every file [depends: 10]

## DONE
