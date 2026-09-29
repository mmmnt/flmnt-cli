# ketchup-plan — flmnt 1.10.4 full-surface verification (2026-09-28)

Founder: "flmnt updated. verify all commands updated/functional." Swept all 21 commands and 15
subcommands from `--help` itself rather than a hand-list, then exercised each against production.

## TODO
- [ ] Burst 1: `flmnt dashboard` opens the dashboard for the environment you are signed in to [depends: none]
- [ ] Burst 2: every command that resolves an auth server URL registers --server-url [depends: 1]

## FOUND BY THE SWEEP
- `corpus` rendered a single 1090-section rulings.md for a workspace with NO DOC-NODE instead of
  refusing — and the refusal message I wrote was unreachable dead code, because rulings.md is
  created whenever any decision is unwired, which is every real project stream. FIXED b0fb92b.
- `dashboard` opens `http://localhost:3001` — `browser.ResolveURL()` reads QUORUM_DASHBOARD_URL and
  otherwise hardcodes the local stack, ignoring the login config every other command resolves. For
  anyone signed in to production, which is everyone, the command opens a dead URL. Its test ASSERTS
  the localhost default, so the gate blessed it. This is the whoami defect of v1.10.2 unfixed in a
  sibling — exactly what CLAUDE.md means by auditing every sibling the same hour.

## VERIFIED FUNCTIONAL against production
whoami · workspace list (17 workspaces, `platform` resolves by name) · mcp auth-header · brief
(picks up the new keyframe) · gate (silent when fresh, nudges at --threshold 1s) · health (reports
the local stack down, exit 1) · corpus (by id AND by name; 98 sections + 29 retired = 127) ·
derive --dry-run (7 sessions, 65 decisions) · sync pull/push --dry-run (exit 1, blocked on the local
broker being down — environmental, message names the cause) · setup (throwaway repo: 12 commands, no
hydrate leftovers, no dead grant, project resolved by NAME) · record-metric + record-attestation
(two metric.recorded entries read back) · record-plan (read back at position 107) ·
record-supersession (live in the SANDBOX workspace per the precedent of exploration 101bb301;
SUPERSEDED_BY edge confirmed, actor stamped clientId=flmnt-cli) · proxy (binds, authenticated:true
against mcp.production) · completion bash/zsh/fish/powershell + dynamic __complete offering corpus
and its flags · guards on record-supersession and record-plan refuse with exit 1.

NOT RUN, deliberately: `login` and `logout` would revoke or re-mint the credentials this session is
using. Registration and flags verified; the live paths are exercised implicitly — every command above
authenticated with the token login produced.

## DONE
