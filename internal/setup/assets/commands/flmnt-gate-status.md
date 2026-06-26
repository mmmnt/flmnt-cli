---
description: Check whether the pre-deploy gate will block, and clear it
argument-hint:
---
Check the pre-deploy gate before attempting a `cdk deploy` / `serverless deploy` / `git push`.

The gate (`scripts/flmnt-predeploy-gate.sh`) BLOCKS those commands unless a flmnt read happened in the last 30 minutes AND `pnpm turbo typecheck lint` is green.

1. Show the marker freshness: run `ls -la ~/.cache/quorum-predeploy/ 2>/dev/null` and report the newest `ack-*` file's age (the gate needs it <30 min).
2. If stale/absent, clear it by consulting the operational rules now: `search_events` on the `{projectId}::operational` stream for "deploy runbook" and "pre-push quality gates" (or `materialize_context`). That read drops a fresh marker via the PostToolUse hook.
3. Remind: the gate also runs `pnpm turbo typecheck lint` — make sure those are green before deploying.
