---
description: Ingest a document/artifact into flmnt (hydrate_artifact + poll status)
argument-hint: [path or description of the artifact]
---
Hydrate a large document (runbook, design doc, spec, log) into flmnt so it becomes retrievable context.

1. Read/obtain the artifact content for: **$ARGUMENTS**.
2. Call `hydrate_artifact(content='<the document text>', content_type='text')` — it returns `{job_id, status}`.
3. Poll `get_hydration_status(job_id)` until complete before relying on the ingested content in `materialize_context` / `query_context`. If it errors, report the degraded state rather than retrying in a loop.
