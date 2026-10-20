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

Actor constraint (coordinator, 2026-10-19): the helper's token carries the ACTOR — the harness mints
sub=local-test-user-0001 for Claude Code's own client id, and every write is attributed from the
router's X-Auth-Sub / X-Auth-Client. The CLI's hosted writes carry its own client id `flmnt-cli`, so
the CLI must ask the helper for a token minted for `flmnt-cli`: it runs the helper with a client-id
variable in its environment (shape pending the quorum side's confirmation of the exact name). No other
client id is named anywhere in this repo.

## Bursts

- [ ] Burst 1: `setup.HeadersHelper(dir)` reads the `flmnt` entry's headersHelper from `.mcp.json` [depends: none]
- [ ] Burst 2: `isLoopback` names a loopback server URL (shared with `secureEndpoint`) [depends: none]
- [ ] Burst 3: `health` probes the configured server's `/health`, names the environment it resolved, and probes core/engine/proxy only when CORE_URL / ENGINE_URL / QUORUM_PROXY_PORT are declared [depends: 2]
- [ ] Burst 4: `setup` preserves a loopback `flmnt` entry that carries a headersHelper; a test round-trips quorum's `.mcp.json` shape [depends: 1,2]
- [ ] Burst 5: `runAuthHelper` returns the helper's Authorization AND X-Workspace-Id, and runs the helper with the CLI's client id in its environment [depends: none]
- [ ] Burst 6: the helper command resolves flag → `.mcp.json` headersHelper → the project-gated default [depends: 1]
- [ ] Burst 7: `bestEffortBearer` tries the helper first on a loopback server and second elsewhere; `--auth-cmd` is a root persistent flag; helper output is memoized per command [depends: 2,5,6]
- [ ] Burst 8: `workspace` commands authenticate through the same bearer path [depends: 7]
- [ ] Burst 9: `whoami` decodes the helper's bearer on a loopback server instead of saying "Not logged in." [depends: 7]
- [ ] Burst 10: on a loopback server the helper's workspace beats the machine-wide active workspace as the project fallback [depends: 7]
- [ ] Burst 11: `sync --local-workspace` defaults to the helper's X-Workspace-Id, then the old "quorum" [depends: 5,6]
- [ ] Burst 12: README documents the local path beside the remote one [depends: 1-11]

## Left open (founder decisions — not guessed)

- The harness publishes only the MCP (`:8000`); the router (`flmnt-router:4000/graphql`) is in-network
  only and the MCP's discovery document 404s there, so every GraphQL command needs
  `FILAMENT_GRAPHQL_ENDPOINT` (or `--graphql-endpoint`) pointing at a host-reachable router. Either the
  compose publishes `127.0.0.1:4000` or the MCP proxies `/graphql`; that is a quorum-side change.
- `setup --server-url <hosted>` inside a checkout whose `flmnt` entry is loopback + headersHelper keeps
  that entry and says so. There is no flag to force the overwrite ("unless asked" is undefined).
- The name of the client-id variable the CLI sets on the helper's environment (burst 5).
