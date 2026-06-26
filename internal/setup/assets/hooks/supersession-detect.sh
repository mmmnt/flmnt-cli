#!/usr/bin/env bash
# UserPromptSubmit hook: when the user's message reverses or replaces an earlier decision, nudge the
# model to author a record_supersession so retrieval serves the superseder (not the stale decision)
# and the dashboard's Decision Stability KPI reflects the change. A hook can't call MCP — it injects
# context; the model makes the call. (Implemented as a prompt-regex nudge rather than a PreToolUse
# Edit/Write gate, which would fire on every file edit — far too noisy.)

input=$(cat)

# Revision / replacement language anywhere in the prompt.
if printf '%s' "$input" | grep -qiE "\b(instead of|rather than|replace .* with|switch (to|from)|no longer (use|using|going)|let'?s not .* anymore|change(d)? (our|my|the) mind|on second thought|scrap (that|the)|abandon (that|the)|supersede|deprecate|reverse (the|our) (decision|call)|use .* not |go with .* instead|undo (the|our) (decision|approach))\b"; then
  cat <<'EOF'
[Supersession capture] This reverses or replaces an earlier decision. If a recorded decision is now stale, author the replacement structurally: find the prior decision's id (peek_stream / search_events on the domain or operational stream), then call record_supersession(stream_id='<new decision's stream>', content='<the new decision>', supersedes='<prior decision id>'). This creates the SUPERSEDED_BY edge so the stale one is never served as current. Only for genuine decision reversals — not routine edits.
EOF
fi
exit 0
