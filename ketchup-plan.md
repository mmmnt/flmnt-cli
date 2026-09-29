# ketchup-plan — the corpus render says how current it is, and refreshes itself

Founder ruling 2026-09-28 (both options): stamp the render with its high-water mark AND refresh it
from a SessionStart hook. Closes the quiet half of the "generated, never hand-maintained" ruling —
the files were derived but only when somebody remembered to derive them.

## Design
- The corpus lives in a DIFFERENT workspace from the repo's own (quorum records into bc674142; the
  doctrine is in d6dd5b65). So the hook cannot reuse `--project`; setup needs to be told which
  workspace holds the corpus, and wires the hook only when it is told.
- The hook must fail QUIET like `brief` and `gate` — offline, not logged in, or a workspace with no
  DOC-NODE must leave the files alone and say nothing, never break a session start.

## DONE
- [x] Burst 1: the generated header records the newest entry the render came from
- [x] Burst 2: `flmnt corpus --hook` fails quiet and writes nothing when it cannot render
- [x] Burst 3: `flmnt setup --corpus-project` records which workspace holds the corpus
- [x] Burst 4: setup wires the SessionStart refresh only when a corpus workspace is recorded
- [x] `--corpus-project` registered on setup and resolved by NAME

VERIFIED END TO END in a throwaway repo, not by flag-shape assertion:
`setup --project quorum --corpus-project platform` resolved both names, wrote
`corpus_project: d6dd5b65-…` beside `project_id: bc674142-…`, and wired
`flmnt corpus --hook --project d6dd5b65-…` as the third SessionStart hook. Running that hook
rendered five documents silently (exit 0); pointing it at a workspace with no DOC-NODE exited 0 and
wrote nothing. Header now reads: "Rendered from 187 entries; newest `fac52696-…` at 2026-09-28T23:23:18.961Z."

## Not building, and why
A separate drift detector that compares the file's stamp against the live stream. With the hook
refreshing every session start, the only window it could report is one the next session closes — and
the stamp already lets a reader judge age without a tool.

## DONE
