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

Identity contract (founder-ruled, revised final + addendum, 2026-10-19 — replaces every earlier
note): the CLI owns a LOCAL LOGIN and the quorum helper only consumes it. `flmnt login --local` takes
no input: it reads the host marker (macOS IOPlatformUUID / Linux machine-id / Windows MachineGuid)
and the OS user, derives sub = "local_" + UUIDv5(Namespace, marker + "\n" + osUser), prints it, and
writes ~/.filament/local-identity.json (0600) holding {sub, created_at} — never the marker; refuses
when no marker can be read. `flmnt logout --local` removes the file. client_id = "local_" +
UUIDv5(sub, tool). `flmnt whoami --json [--client <tool>]` prints {sub,workspace,client_id?}:
loopback → local login (absent → exit 1, "run flmnt login --local"), hosted → keychain (client_id =
the registered OAuth client). Plain whoami says "local identity <sub>". The CLI runs the helper with
`--client flmnt-cli` appended; no BROKER_CLIENT_ID. Fail closed; refusals surfaced verbatim.

## Bursts

- [x] Burst 1: `setup.HeadersHelper(dir)` reads the `flmnt` entry's headersHelper from `.mcp.json` [depends: none]
- [x] Burst 2: `isLoopback` names a loopback server URL (shared with `secureEndpoint`) [depends: none]
- [x] Burst 3: `health` probes the configured server's `/health`, names the environment it resolved, and probes core/engine/proxy only when CORE_URL / ENGINE_URL / QUORUM_PROXY_PORT are declared [depends: 2]
- [x] Burst 4: `setup` preserves a loopback `flmnt` entry … — REVERSED by founder ruling ("setup should override the flmnt key no matter what"): the entry is always rewritten; other servers verbatim [depends: 1,2]
- [x] Burst 5: `runAuthHelper` returns the helper's Authorization AND X-Workspace-Id, runs it with BROKER_CLIENT_ID=flmnt-cli, and carries its stderr line in the error [depends: none]
- [x] Burst 6: the helper command resolves flag → `.mcp.json` headersHelper → the project-gated default [depends: 1]
- [x] Burst 7: `bestEffortBearer` tries the helper first on a loopback server and second elsewhere; `--auth-cmd` is a root persistent flag; helper output is memoized per command [depends: 2,5,6]
- [x] Burst 8: `workspace` commands authenticate through the same bearer path [depends: 7]
- [x] Burst 9: `whoami --json [--client <tool>]` — local login on a loopback server, stored login elsewhere; built once against the keychain, removed on the hold, rebuilt on the final contract [depends: 15]
- [x] Burst 10: `whoami` on a loopback server decodes the helper's bearer instead of saying "Not logged in." [depends: 7]
- [x] Burst 11: on a loopback server the helper's workspace beats the machine-wide active workspace as the project fallback [depends: 7]
- [x] Burst 12: `sync --local-workspace` defaults to the helper's X-Workspace-Id, then the old "quorum" [depends: 5,6]
- [x] Burst 13: `health` on a loopback server reports the helper: the minted actor, or the helper's reason [depends: 3,7]
- [x] Burst 14: README documents the local path beside the remote one [depends: 1-13,15,16]
- [x] Burst 15: `internal/identity` — local_-prefixed UUIDv5 derivation under a fixed namespace, the host marker readers, the digest-only login file [depends: none]
- [x] Burst 16: `login --local` (no input) records the derived sub, `logout --local` removes it; refuses without a marker [depends: 15]
- [x] Burst 17: the helper runs with `--client flmnt-cli` (BROKER_CLIENT_ID dropped) [depends: 5]
- [x] Burst 18: on a loopback server the checkout's `local_graphql_endpoint` (.quorum.json) is the GraphQL endpoint before `<server>/graphql`; setup preserves the field; the `--client` strip is gone (the helper takes the last one) [depends: 2]

## Left open (founder decisions — not guessed)

- CLOSED (founder): the harness publishes the router at 127.0.0.1:4000 and the checkout declares it as
  `local_graphql_endpoint` in .quorum.json (burst 18).
- CLOSED (founder): setup overrides the `flmnt` key no matter what.
- CLOSED: the helper takes the LAST `--client`, so the CLI appends its own and parses nothing.
- CLOSED (founder): hosted `whoami --json --client` reports the registered OAuth client id as is.

## Bottle 5.3 — the helper retires (QRM-ADR-015 §1, §6)

quorum's `scripts/mcp-auth-header.sh` is gone and `.mcp.json` is url-only, local and hosted alike. The
CLI's harness credential is the keychain token `flmnt login --local` stores (Bottle 5.2), read exactly
as the hosted login's; nothing runs a helper any more.

- [x] Burst 19: `bestEffortBearer` reads the keychain only, for a loopback server too [depends: 7]
- [x] Burst 20: a server with no stored token is refused with the login that fills it — `flmnt login --local --server-url <server>` for the harness [depends: 19]
- [x] Burst 21: `sync`'s local side authenticates from the keychain; `--local-auth-cmd` goes; `--local-workspace` defaults to "quorum" [depends: 19,20]
- [x] Burst 22: `health` on a loopback server reports the stored local token (who it is for, or the refusal) instead of running the helper [depends: 20]
- [x] Burst 23: the project chain is the same on every server — flag, repo pin, active workspace; `helperWorkspace` goes [depends: 11]
- [x] Burst 24: `runAuthHelper`, `localCredentials`, `--auth-cmd`, `resolveAuthCmd`, `setup.FlmntEntry` removed [depends: 19-23]
- [x] Burst 25: the url-only `.mcp.json` entry is asserted for both environments; README/npm README no longer name a helper [depends: 24]
