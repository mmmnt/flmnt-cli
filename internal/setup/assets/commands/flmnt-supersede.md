---
description: Replace a prior decision with a SUPERSEDED_BY edge (record_supersession)
argument-hint: [the new decision; old one will be located]
---
A prior decision is now stale — author the replacement structurally so retrieval never serves the old one as current.

1. Find the prior decision's entry id: `search_events` / `peek_stream` on the `::domain` or `::operational` stream for the decision being replaced.
2. Call `record_supersession(stream_id='<new decision's stream>', content='<the new decision: $ARGUMENTS>', supersedes='<prior decision id>')`. This records the new decision AND creates the typed `SUPERSEDED_BY` edge (cross-stream capable), lifting the Decision Stability KPI.

Use this for genuine reversals/replacements, not routine edits.
