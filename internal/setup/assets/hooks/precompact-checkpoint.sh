#!/usr/bin/env bash
# PreCompact hook: context is about to be summarized/discarded — the highest-value moment to persist
# state. Nudge the model to checkpoint open threads + the live plan before they're lost. This runs
# alongside `flmnt derive --write` (wired in settings.json) which deterministically derives and writes
# decisions/keyframes/mistakes from the transcript+git as a backstop that needs no model compliance.
#
# Stream ids are STRICT ('{workspaceId}::{type}') and must be resolved from the live workspace via
# list_streams — a fabricated id (e.g. derived from a filesystem path) makes every stream-scoped call
# fail with workspace_not_found. QUORUM_PROJECT_ID is honored only when explicitly configured.

if [[ -n "$QUORUM_PROJECT_ID" ]]; then
  DOMAIN_REF="'${QUORUM_PROJECT_ID}::domain'"
  PLAN_REF="record_plan('${QUORUM_PROJECT_ID}::plan', ...)"
else
  DOMAIN_REF="the workspace's domain stream (resolve the exact streamId via list_streams first — never guess one or derive it from a filesystem path)"
  PLAN_REF="record_plan on the resolved plan stream"
fi

cat <<EOF
[flmnt pre-compaction checkpoint] Context is about to be compacted. Before it is, persist anything a future window would otherwise lose: call write_keyframe on ${DOMAIN_REF} with the current understanding + open threads, ${PLAN_REF} if a multi-step plan is in flight, and record_decision / record_mistake for any unrecorded conclusions or errors. (A deterministic 'flmnt derive --write' also runs on this event as a backstop.)
EOF
exit 0
