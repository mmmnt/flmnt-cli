---
description: Checkpoint current session understanding to flmnt (write_keyframe)
argument-hint: [optional focus/note]
---
Checkpoint the current stable understanding so a future window can resume without re-deriving it.

Call `write_keyframe` on the project's `::domain` stream with a concise prose summary of: what you now understand, the current state, decisions made, and open threads$ARGUMENTS. Pass `causal_refs` = the entry ids that informed this checkpoint (pull them from a recent `peek_stream`/`materialize_context` if available). Write one good keyframe for the stable state — not speculative or trivial ones.
