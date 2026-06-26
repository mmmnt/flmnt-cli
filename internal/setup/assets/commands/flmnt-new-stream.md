---
description: Provision a new flmnt stream (create_stream)
argument-hint: <stream_type> <name>
---
Provision a new stream: **$ARGUMENTS**

1. First `list_streams` to confirm it doesn't already exist (don't duplicate; `domain`/`operational`/`mistake` are per-project singletons provisioned automatically — don't recreate them).
2. Call `create_stream(stream_type='<type>', name='<name>', topology={...})`. For a per-session keyframe stream pass `topology={session_id: "..."}`; for a metrics stream create it once per project.
3. Report the new `stream_id`; resolve it via `list_streams` in future rather than hardcoding.
