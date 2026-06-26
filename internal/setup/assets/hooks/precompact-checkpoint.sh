#!/usr/bin/env bash
# PreCompact hook: context is about to be summarized/discarded — the highest-value moment to persist
# state. Nudge the model to checkpoint open threads + the live plan before they're lost. This runs
# alongside `flmnt derive --write` (wired in settings.json) which deterministically derives and writes
# decisions/keyframes/mistakes from the transcript+git as a backstop that needs no model compliance.

if [[ -z "$QUORUM_PROJECT_ID" ]]; then
  _root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
  QUORUM_PROJECT_ID="${_root//\//-}"
fi

cat <<EOF
[flmnt pre-compaction checkpoint] Context is about to be compacted. Before it is, persist anything a future window would otherwise lose: call write_keyframe on '${QUORUM_PROJECT_ID}::domain' with the current understanding + open threads, record_plan('${QUORUM_PROJECT_ID}::plan', ...) if a multi-step plan is in flight, and record_decision / record_mistake for any unrecorded conclusions or errors. (A deterministic 'flmnt derive --write' also runs on this event as a backstop.)
EOF
exit 0
