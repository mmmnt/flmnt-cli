#!/usr/bin/env bash
# PreCompact hook: context is about to be summarized/discarded — the highest-value moment to persist
# state. Nudge the model to checkpoint open threads + the live plan before they're lost. This runs
# alongside `flmnt derive --write` (wired in settings.json) which deterministically derives and writes
# decisions/keyframes/mistakes from the transcript+git as a backstop that needs no model compliance.
#
# Stream ids are STRICT and must be resolved from the live workspace via list_streams — a fabricated id
# (e.g. derived from a filesystem path) makes every stream-scoped call fail with workspace_not_found.
# FLMNT_PROJECT_ID (or its former name, QUORUM_PROJECT_ID) is honored only when explicitly configured.
# A keyframe goes to the session's keyframe stream, {project}::{session_id}::keyframe; the session id is
# the one the hook JSON on stdin names, the same id `flmnt session-start` named when the session began.

HOOK_INPUT=""
[[ -t 0 ]] || HOOK_INPUT=$(cat)
SESSION_ID=$(printf '%s' "$HOOK_INPUT" \
  | sed -n 's/.*"session_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
  | head -1)

PROJECT_ID="${FLMNT_PROJECT_ID:-${QUORUM_PROJECT_ID:-}}"
if [[ -n "$PROJECT_ID" && -n "$SESSION_ID" ]]; then
  KEYFRAME_REF="'${PROJECT_ID}::${SESSION_ID}::keyframe'"
else
  KEYFRAME_REF="this session's keyframe stream (the stream_id the session-start hook named)"
fi
if [[ -n "$PROJECT_ID" ]]; then
  PLAN_REF="record_plan('${PROJECT_ID}::plan', ...)"
else
  PLAN_REF="record_plan on the plan stream (resolve the exact streamId via list_streams first — never guess one or derive it from a filesystem path)"
fi

cat <<MSG
[flmnt pre-compaction checkpoint] Context is about to be compacted. Before it is, persist anything a future window would otherwise lose: call write_keyframe on ${KEYFRAME_REF} with the current understanding + open threads, ${PLAN_REF} if a multi-step plan is in flight, and record_decision / record_mistake for any unrecorded conclusions or errors. (A deterministic 'flmnt derive --write' also runs on this event as a backstop.)
MSG
exit 0
