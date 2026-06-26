#!/usr/bin/env bash
# UserPromptSubmit hook: when the user's message looks like a correction of a mistake,
# remind Claude to persist it via the record_mistake MCP tool (which routes to the dedicated
# {projectId}::mistake stream). A hook cannot call an MCP tool itself — it injects context and
# the model makes the call, mirroring scripts/keyframe-gate.sh and scripts/mcp-search-reminder.sh.

input=$(cat)

# Match common correction / error signals (case-insensitive) anywhere in the prompt payload.
if printf '%s' "$input" | grep -qiE "that('?s| is) (wrong|not right|incorrect)|you broke|broke (it|the)|\brevert\b|\bundo\b|roll ?back|still (failing|broken|not working)|did(n'?t| not) work|not what i (asked|wanted|meant)|no,? (that|it|you)|wrong (file|approach|place|answer)|regression|that'?s not (it|right|correct)|\bmistake\b|messed up|screwed up|why did you"; then
  cat <<'EOF'
[Mistake capture] The user's message looks like a correction of something that went wrong. If you made an error this session, persist it so it surfaces on the dashboard: call record_mistake(content='<concise what went wrong + the fix>', was_corrected=<true if already fixed, else false>, target_entry_id='<id of the recorded decision/keyframe it stemmed from, if any>', corrects='<id of a prior recorded mistake this corrects, if any>'). It routes to the {projectId}::mistake stream. Record only genuine mistakes you made — not user preference changes or new feature requests.
EOF
fi
exit 0
