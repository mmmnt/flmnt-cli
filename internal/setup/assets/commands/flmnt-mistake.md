---
description: Record a genuine mistake + its fix to flmnt (feeds Mistake Correction Rate)
argument-hint: [what went wrong + the fix]
---
Record a genuine error made this session so it surfaces on the dashboard.

Call `record_mistake(content='<concise: what went wrong + the fix>'$ARGUMENTS, was_corrected=<true if already fixed else false>, target_entry_id='<id of the decision/keyframe it stemmed from, if any>', corrects='<id of a prior recorded mistake this corrects, if any>')`. It routes to the `{projectId}::mistake` stream and drives the Mistake Correction Rate KPI. Record only real mistakes you made — not user preference changes or new requests. Use `corrects` to flip an earlier-recorded mistake to corrected once fixed.
