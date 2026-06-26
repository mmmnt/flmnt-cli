---
description: Persist a multi-step implementation plan to flmnt (record_plan)
argument-hint: [goal / plan summary]
---
Persist the current multi-step plan so it survives a context reset and can be recovered with `read_plan` next session.

Call `record_plan(stream_id='<project>::plan', content='<the burst-by-burst / step-by-step plan>')` capturing: the goal$ARGUMENTS, the ordered steps, and any per-step verification. The `::plan` stream has a deterministic id (`{projectId}::plan`) — reuse it. If you don't have a structured plan yet, draft one first, then record it.
