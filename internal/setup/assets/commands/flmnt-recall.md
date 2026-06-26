---
description: Recall recorded context from flmnt before deep work (causal retrieval, not grep)
argument-hint: [what you're trying to understand]
---
Before searching the codebase, recall what flmnt already knows about: **$ARGUMENTS**

1. Call `query_context` with that as the query (query-driven causal retrieval — it orients, fans out the causal graph, ranks, and surfaces recent mistakes around the landing point).
2. If you need breadth, also `search_events` on the project's `::domain` / `::operational` / `::mistake` streams, and `get_graph_neighborhood` on the most relevant entry to see its causes/effects.
3. Summarize what you found (decisions, root causes, runbooks, prior mistakes) and only fall back to reading/grepping code for what the streams don't cover.
