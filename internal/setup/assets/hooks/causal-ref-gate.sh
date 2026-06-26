#!/usr/bin/env bash
# PreToolUse(mcp__flmnt__record_decision) gate — HARD-BLOCK a planning decision that links no
# causal_refs and does not explicitly acknowledge their absence. CLAUDE.md's causal-ref gate is
# enforced server-side (the tool returns {"status":"causal_ref_required"} and records nothing), but
# that wastes a round-trip and lets bare decisions slip when the model passes acknowledge by reflex.
# Blocking here forces the model to either link the entries that caused the decision (lifting the
# dashboard's Causal Density KPI) or consciously acknowledge none apply.
#
# Exit 0 = allow; exit 2 = block (stderr is shown to the agent).
input=$(cat)

verdict=$(printf '%s' "$input" | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception:
    print("allow"); sys.exit(0)
ti = d.get("tool_input") or {}
refs = ti.get("causal_refs")
ack = ti.get("acknowledge_no_causal_ref")
has_refs = isinstance(refs, list) and len(refs) > 0
ack_true = ack is True or (isinstance(ack, str) and ack.strip().lower() in ("true", "1", "yes"))
print("allow" if (has_refs or ack_true) else "block")
' 2>/dev/null) || exit 0

if [ "$verdict" = "block" ]; then
  cat >&2 <<'EOF'
[Causal-ref gate] BLOCKED — record_decision needs its causal links.
A planning decision must cite the entries that caused it. Do one of:
  • pull the ids that informed this decision (peek_stream / slice_events / get_graph_neighborhood
    on the relevant stream) and re-call record_decision with causal_refs=[those ids]; or
  • if genuinely no prior entry applies, re-call with acknowledge_no_causal_ref=True.
This keeps the domain stream's causal graph dense (dashboard Causal Density KPI).
EOF
  exit 2
fi
exit 0
