# ketchup-plan — flmnt 1.10.4 full-surface verification (2026-09-28)

Founder: "flmnt updated. verify all commands updated/functional." Swept all 21 commands and 15
subcommands from `--help` itself rather than a hand-list, then exercised each against production.

## DONE — three defects, all TCR'd, shipped as v1.10.5
- [x] Burst 1: `flmnt dashboard` opens the dashboard for the environment you are signed in to
- [x] Burst 2: gate and sync accept the --server-url they already resolve, swept not listed
- [x] Burst 3: the server URL falls back to the login already recorded

## FOUND BY THE SWEEP
- `corpus` rendered a single 1090-section rulings.md for a workspace with NO DOC-NODE instead of
  refusing — and the refusal message I wrote was unreachable dead code, because rulings.md is
  created whenever any decision is unwired, which is every real project stream. FIXED b0fb92b.
- `gate --server-url` died with "unknown flag" though gate RESOLVES a server URL — the v1.10.3
  --project defect one flag over, and gate runs inside a UserPromptSubmit hook, so it is a hard
  failure there. `sync` had it too, while its own comment described --server-url as part of its
  resolution chain. Both fixed (cdc9cb2) behind a sweep over cmd/*.go that fails on any file
  resolving a server URL without registering the flag — verified by reintroducing the defect.
- Outside a configured repo, with a valid login on disk, EVERY command that needs a server URL
  refused: whoami answered "--server-url or QUORUM_SERVER_URL is required" and dashboard silently
  opened the local stack. `resolveActiveWorkspace` already falls back to the login config for the
  workspace; the server URL never did, so the two halves of "where am I" resolved from different
  places. Fixed (bad0c4e) as the last link in the chain, so a repo still wins.
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
