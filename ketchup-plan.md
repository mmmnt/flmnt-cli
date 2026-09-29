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

## TODO
- [ ] Burst 1: the generated header records the newest entry the render came from [depends: none]
- [ ] Burst 2: `flmnt corpus --hook` fails quiet and writes nothing when it cannot render [depends: none]
- [ ] Burst 3: `flmnt setup --corpus-project` records which workspace holds the corpus [depends: none]
- [ ] Burst 4: setup wires the SessionStart refresh only when a corpus workspace is recorded [depends: 2, 3]

## Not building, and why
A separate drift detector that compares the file's stamp against the live stream. With the hook
refreshing every session start, the only window it could report is one the next session closes — and
the stamp already lets a reader judge age without a tool.

## DONE
