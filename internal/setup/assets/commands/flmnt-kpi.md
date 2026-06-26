---
description: Snapshot flmnt recording health (streams + KPIs) and flag gaps
argument-hint:
---
Give a quick health snapshot of the project's recording discipline and the dashboard KPIs it feeds.

1. `list_streams` then `get_stream_metadata` on `::domain`, `::operational`, `::mistake`, `::plan`, `::metrics` — report entryCount + last-write recency for each.
2. `peek_stream` the latest few from `::domain` and `::mistake` to gauge freshness.
3. Map to the dashboard KPIs and flag weak ones:
   - **Causal Density** ← `record_decision` with `causal_refs` (are decisions linking causes?)
   - **Mistake Correction Rate** ← `record_mistake` `was_corrected`/`corrects` (are caught mistakes being closed?)
   - **Decision Stability** ← `record_supersession` edges (are reversals recorded structurally?)
   - **Cache Savings** ← `materialize_context`/`query_context` usage
4. Recommend which write tool is under-used this session. For the full dashboard, suggest `flmnt dashboard`.
