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

## DONE — `flmnt corpus`, shipped
Every burst TCR'd, 17 commits. Verified against the real corpus (d6dd5b65::domain, 187 entries):

    the-methodology.md                 28 sections
    system-architecture-reference.md   19 sections
    specification-testing-reference.md 13 sections
    ndd-palette-the-visual-workshop.md  7 sections
    rulings.md                         31 sections
    not doctrine, rendered nowhere: exploration.committed 4, keyframe.written 50,
                                    plan.snapshot 1, system.stream_created 1

THE PROOF THAT NOTHING IS LOST: 98 sections + 29 appendix entries = 127, which is exactly the
doctrine the stream holds (99 decision.made + 32 decision.superseded − 4 DOC-NODE). That equality
is a test, not an observation — TestEveryDoctrineEntryRendersExactlyOnce.

THREE RULINGS THE FIRST DESIGN SILENTLY DROPPED, found by checking the count instead of reading
the output:
- Two entries are each superseded TWICE (deliberate amendments to different clauses of §5c and
  §1d). A one-superseder-per-target map kept whichever came last and lost the other.
- One supersession targets a KEYFRAME, so it belonged to no section and rendered nowhere — and it
  was not even counted as unrendered, because supersessions were exempt from that count.
Both fixed by rooting each entry in its own supersession lineage: a ruling whose target is not
doctrine roots at itself, and branches render side by side in the original's position.

ALSO FIXED HERE: the --project sweep test from 852f118 ENUMERATED four commands while its comment
claimed to sweep. `corpus` proved the point by not being covered. It now sweeps cmd/*.go for
`resolveProject(cmd` and fails on any file that resolves a project without registering the flag —
verified by deleting corpus.go's flag registration and watching it fail by file name.
