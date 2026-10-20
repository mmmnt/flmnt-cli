# ketchup-plan — the CLI works against the local harness without losing the hosted path

Founder directive: "catch the CLI up with the recent changes ... make sure the CLI work doesn't remove
remote functionality, we need to extend local support. I think the CLI has some of that work already
that can be extended."

Contract (quorum repo, 2026-10-19): `scripts/env.sh` resolves `local | localstack | aws`;
`scripts/flmnt-cli.sh` runs the hooks' CLI with `QUORUM_SERVER_URL=http://localhost:8000/mcp` in
`local`; `scripts/mcp-auth-header.sh` (the `.mcp.json` headersHelper) prints
`{"Authorization":"Bearer <offline-minted RS256>","X-Workspace-Id":"<workspace>"}` in `local` and
delegates to `flmnt mcp auth-header` in `aws`. The harness publishes ONLY `127.0.0.1:8000` (the MCP).

What the CLI already had: `sync`'s local side runs a headers-helper (`runAuthHelper`) gated on a
`.quorum.json`. Every other command authenticated from the keychain only, so against the harness it
went quiet. The helper path becomes the shared local-credential path; the keychain/OAuth path is
untouched and stays first for a non-loopback server.

Actor ruling (founder, 2026-10-19): the harness mints as a declared person — no synthetic fallback;
minting BLOCKS when the identity is not declared. The helper (scripts/mcp-auth-header.sh →
scripts/local/mint-token.ts) mints for the client it is given — `--client <id>` as an argument
(Claude Code's .mcp.json entry declares `--client claude-code-local`), else BROKER_CLIENT_ID — and
refuses with a reason when either is undeclared: that is "not configured", the stderr line is
surfaced, and no default is ever substituted.

HELD (coordinator, later the same day): the founder revised the identity contract. A fixed client id
and a reused hosted sub are out — both must be unique and persistent, and nobody can be assumed to
have a sub already. The replacement is a LOCAL LOGIN in the CLI (a declared email plus a host-machine
marker, persisted; a unique sub and per-tool client ids derive from it); the exact contract is to
follow. Consequences in this repo: `whoami --json` (built against the keychain id token, commit
3bb5a7a) was taken back out; the CLI still runs the helper with BROKER_CLIENT_ID=flmnt-cli because
that is what the helper requires TODAY — provisional, to be replaced by the derived client id.

## Bursts

- [x] Burst 1: `setup.HeadersHelper(dir)` reads the `flmnt` entry's headersHelper from `.mcp.json` [depends: none]
- [x] Burst 2: `isLoopback` names a loopback server URL (shared with `secureEndpoint`) [depends: none]
- [x] Burst 3: `health` probes the configured server's `/health`, names the environment it resolved, and probes core/engine/proxy only when CORE_URL / ENGINE_URL / QUORUM_PROXY_PORT are declared [depends: 2]
- [x] Burst 4: `setup` preserves a loopback `flmnt` entry that carries a headersHelper; a test round-trips quorum's `.mcp.json` shape [depends: 1,2]
- [x] Burst 5: `runAuthHelper` returns the helper's Authorization AND X-Workspace-Id, runs it with BROKER_CLIENT_ID=flmnt-cli, and carries its stderr line in the error [depends: none]
- [x] Burst 6: the helper command resolves flag → `.mcp.json` headersHelper → the project-gated default [depends: 1]
- [x] Burst 7: `bestEffortBearer` tries the helper first on a loopback server and second elsewhere; `--auth-cmd` is a root persistent flag; helper output is memoized per command [depends: 2,5,6]
- [x] Burst 8: `workspace` commands authenticate through the same bearer path [depends: 7]
- [~] Burst 9: `whoami --json` — built, then REMOVED on the hold above; the local login replaces it [depends: the local-login contract]
- [x] Burst 10: `whoami` on a loopback server decodes the helper's bearer instead of saying "Not logged in." [depends: 7]
- [x] Burst 11: on a loopback server the helper's workspace beats the machine-wide active workspace as the project fallback [depends: 7]
- [x] Burst 12: `sync --local-workspace` defaults to the helper's X-Workspace-Id, then the old "quorum" [depends: 5,6]
- [x] Burst 13: `health` on a loopback server reports the helper: the minted actor, or the helper's reason [depends: 3,7]
- [x] Burst 14: README documents the local path beside the remote one [depends: 1-13]

## Left open (founder decisions — not guessed)

- The harness publishes only the MCP (`:8000`); the router (`flmnt-router:4000/graphql`) is in-network
  only and the MCP's discovery document 404s there, so every GraphQL command needs
  `FILAMENT_GRAPHQL_ENDPOINT` (or `--graphql-endpoint`) pointing at a host-reachable router. Either the
  compose publishes `127.0.0.1:4000` or the MCP proxies `/graphql`; that is a quorum-side change.
- `setup --server-url <hosted>` inside a checkout whose `flmnt` entry is loopback + headersHelper keeps
  that entry and says so. There is no flag to force the overwrite ("unless asked" is undefined).
- The local-login contract (identity + derived client ids). Until it lands the CLI mints through the
  helper as BROKER_CLIENT_ID=flmnt-cli, and the entry's `--client` argument is dropped when the CLI
  runs the checkout's helper (`withoutClientArg`) — the CLI parses that one argument of the helper's.
- The helper's authoritative client declaration: today the `--client` ARGUMENT beats the environment,
  which is why the CLI strips it. If the founder would rather the environment win for the CLI, the
  strip goes.
