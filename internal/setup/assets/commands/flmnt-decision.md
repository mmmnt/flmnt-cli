---
description: Record a planning/architecture decision to flmnt (causal-ref gated)
argument-hint: <the decision, as a statement of fact>
---
Record this planning/architecture/strategy decision: **$ARGUMENTS**

1. First gather the causal links: `peek_stream` / `slice_events` / `get_graph_neighborhood` on the `::domain` stream to find the entry ids that led to this decision.
2. Call `record_decision(stream_id='<project>::domain', content='<decision phrased as a fact>', causal_refs=[those ids])`.
3. The causal-ref gate is HARD-ENFORCED by a hook — a call with no `causal_refs` and no `acknowledge_no_causal_ref` is blocked. Only if genuinely no prior entry applies, pass `acknowledge_no_causal_ref=True`.

Phrase it as a commitment ("Use X for Y"), not a maybe. For deploy/ops/infra decisions use `/flmnt-ops` instead.
