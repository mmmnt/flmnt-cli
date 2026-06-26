#!/usr/bin/env bash
# UserPromptSubmit hook: when the user's message describes multi-step / sequential work, nudge the
# model to persist a plan via record_plan so it survives a context reset (and can be recovered with
# read_plan next session). A hook can't call MCP — it injects context; the model makes the call.
# Mirrors scripts/mistake-capture-reminder.sh.

input=$(cat)

# Multi-step intent signals: numbered/bulleted lists, sequencing words, explicit planning language.
if printf '%s' "$input" | grep -qiE "\b(then|after that|next,|afterwards|followed by|step [0-9]|phase [0-9]|first .* then|once .* (then|done))\b|[0-9]\.[[:space:]].+[0-9]\.[[:space:]]|\b(plan to|let'?s (build|implement|do) (this|it) out|break (this|it) (down|into)|multi-?step|several steps|a few things)\b"; then
  cat <<'EOF'
[Plan capture] This looks like multi-step work. Before executing, persist the plan so it survives a context reset: call record_plan(stream_id='{projectId}::plan', content='<the burst-by-burst / step-by-step plan>'). Recover it next session with read_plan instead of re-deriving. (For a one-off single step, skip this.)
EOF
fi
exit 0
