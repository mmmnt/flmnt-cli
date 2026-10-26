# flmnt

The developer CLI for the [flmnt](https://flmnt.dev) event-stream memory platform — authentication, workspace management, and the MCP auth helper for connecting clients like Claude Code to a live flmnt MCP.

## Install

### Shell (macOS / Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/mmmnt/flmnt-cli/main/install.sh | sh
```

### PowerShell (Windows)

```powershell
irm https://raw.githubusercontent.com/mmmnt/flmnt-cli/main/install.ps1 | iex
```

### Homebrew

```sh
brew install mmmnt/tap/flmnt
```

### Scoop

```sh
scoop bucket add flmnt https://github.com/mmmnt/scoop-bucket
scoop install flmnt
```

### npm

```sh
npm install -g @mmmnt/flmnt
```

### Go

```sh
go install github.com/mmmnt/flmnt-cli@latest
```

### Binaries

Prebuilt binaries for macOS, Linux, and Windows (amd64/arm64) are attached to each [release](https://github.com/mmmnt/flmnt-cli/releases).

## Quickstart

Authenticate, pick a workspace, and wire Claude Code to your flmnt MCP:

```sh
flmnt login                                        # OAuth2 — browser PKCE (or --device for headless)
flmnt setup --server-url <flmnt-url> --project <name>   # wire this repo to a workspace
```

`--project` names the workspace **this repo** records into, by name — you do not need its id.
It is required, and it is the whole point: without it a repo falls back to the machine-wide
active workspace, so whichever workspace you last ran `flmnt workspace use` on decides where
every repo's sessions are written. One project's prompts and recaps land in another project's
stream, and `flmnt brief` opens sessions with the wrong project's state.

Once set, the repo is pinned: every hook targets that workspace regardless of what the CLI is
pointed at elsewhere. Setup prints which workspace it chose.

`flmnt setup` writes a project-local `.mcp.json` pointing at the local proxy plus a
`.claude/settings.local.json` UserPromptSubmit hook, then `flmnt proxy` injects your
bearer token on outbound MCP requests — so Claude Code talks to a live, authenticated
flmnt MCP without you handling tokens by hand. `setup` is idempotent.

`flmnt workspace use <name|id>` sets the ACTIVE workspace, which is a per-machine convenience
for ad-hoc commands — it does not decide where a configured repo records.

### Local flmnt (the compose harness)

A checkout can run against a local flmnt instead of the hosted one. **The CLI holds one login at a
time, and the login decides which.** `flmnt login` records a hosted session (the server it signed in
to, who the token says you are, the OAuth endpoints); `flmnt login --local` records a local session
(the loopback server, the derived sub). Either login replaces the other's session entirely — every
stored token for every server goes — and `flmnt logout` ends the session the same way: the config,
every token, the file keyring of older builds. **The local identity is not a login and stays.** It is
who this machine is (the derived `local_` sub); `flmnt login --local` records it once and reuses it
ever after, a hosted login keeps it (a person signed in to the cloud still pushes and links their
local records as it), and no logout removes it. `whoami` answers by the session's kind, never by which
credential happens to exist. Every command takes its upstream from the session, so a logged-in person
never passes `--server-url`; the chain is `--server-url` / `QUORUM_SERVER_URL` / **the login** /
`.flmnt.json` (the repo answers only when nothing is logged in). A loopback host —
`http://localhost:8000/mcp` — is local, anything else is hosted. The hosted path is unchanged:
credentials come from the keychain `flmnt login` filled.

**Identity.** A local flmnt mints credentials for a declared person, never for a default:

```sh
flmnt login --local                           # once per machine and OS user — no input; also obtains the CLI's own local token
flmnt login --local --new                     # mint a new local identity (new salt, new sub); the old sub stays in the history
flmnt local approve <code>                    # approve a client's pending sign-in (claude.ai, Claude Code) for the local login
flmnt local claim <code> --server-url <url>   # from YOUR machine: claim a code shown by a teammate's flmnt for your identity, before they approve
flmnt link                                    # link this machine's local identity to your hosted account (needs both logins); every push does it too
flmnt whoami --json --client flmnt-cli        # the identity as one JSON object, with the client id derived for a tool
flmnt logout --local                          # end a local session (the identity stays)
```

`login --local` reads an identifying marker the machine already carries (macOS `IOPlatformUUID`,
Linux `/etc/machine-id`, Windows `MachineGuid`) and the OS user, and derives, deterministically:
`sub = "local_" + UUIDv5(c0a3f0d2-5b1e-4d7a-9c3e-2f6b8a1d4e70, marker + "\n" + osUser)` and
`client_id = "local_" + UUIDv5(sub's UUID, <tool name>)`. The same machine and user give the same sub
forever, a tool name the same client id forever, and two people or two machines never collide; the
`local_` prefix lets the hosted side recognise a local identity without a lookup. A new identity also
carries a random salt in the derivation (`marker + "\n" + osUser + "\n" + salt`), so no two logins
ever collide — not two people on cloned machine ids, not two logins of one person (QRM-ADR-016 §5g);
identities recorded before the salt keep their sub. Only the digest is kept, in the OS keychain under
its own service `flmnt-identity` (never the tokens' `quorum-cli`, which a logout sweeps), with
`~/.flmnt/local-identity.json` (0600) as a cache of it holding `{sub, created_at, salt?, history?}`,
never the marker. `flmnt login --local --new` mints a new identity on purpose — after a refused
connection on a teammate's harness, or for a fresh start — and keeps the old sub in the identity's
history, which `whoami` shows (`formerly local_…`) and which the link request will carry, so nobody
is ever asked which UUID was theirs. A machine with no readable marker is refused; nothing is
generated at random and nothing is reused from the hosted login. `login --local` takes the loopback
server from `--server-url` / `QUORUM_SERVER_URL` when one is named (a hosted URL is refused), else
`http://localhost:8000`, and records it as the session's server. `whoami --json` prints one object
`{"sub","workspace","client_id"?}`: from the local login when that is the session, from the stored
OAuth login when a hosted one is (`client_id` is then the registered OAuth client); `client_id` only
appears with `--client`. Not logged in (either kind): nothing on stdout, one reason line on stderr, exit 1. Plain
`whoami` says `local identity <sub>` for the local login.

**Signing a client in.** The local harness runs the real OAuth broker. A client that signs in to it
(claude.ai, Claude Code) is shown a code such as `BCDF-GHJK` and the command
`flmnt local approve BCDF-GHJK`. The command asks the broker's admin listener — a loopback-only port
(default `http://127.0.0.1:4012`, `--admin-url`) the edge never routes, so reaching it is the proof
that this is the machine — what the code is waiting for, shows it (`Approve Claude
(https://claude.ai/api/mcp/auth_callback) for sub local_…? [y/N]`; `--yes` answers), and approves it
for the local login: the sub, and the client id derived for the client's registered name (its client
id when it has no name), so that client's writes are attributed to it. A device code (the CLI's own
sign-in) is approved for the client id derived for `flmnt-cli`. No local login refuses before anything
is asked; the broker's refusals (`LOCAL_APPROVAL_UNKNOWN`, `LOCAL_APPROVAL_EXPIRED`,
`LOCAL_APPROVAL_ALREADY_APPROVED`, `LOCAL_SUB_INVALID`, `LOCAL_CLIENT_INVALID`,
`LOCAL_ADMIN_NOT_LOOPBACK`) are surfaced verbatim.

**Converging on your cloud account (QRM-ADR-016).** Nothing is ever rewritten on a record: what you
wrote under your local identity keeps that author forever, in both stores. `flmnt sync push` carries
the identity this machine holds (and the subs it replaced) as `identity`, and the hosted side LINKS
them to the account you are logged in to in its join table before the records land, so wherever it has
to act on a local author — a question, a notification, an erasure — it resolves the person at that
moment. A teammate's `local_` records persist as written and you become their default target until
they link. The push needs both: the hosted login (`flmnt login`) and the identity (`flmnt login --local`,
kept through logins and logouts); when a hosted login has swept the local token, it is minted again
from the identity on the spot. `flmnt link` writes the same rows and pushes nothing — for when you write
to a teammate's local flmnt through a tunnel and someone else pushes the workspace. One hosted account
per identity, ever: a sub another account holds is refused (`LINK_SUB_TAKEN`). `flmnt sync pull`
writes the cloud copy exactly as it is; a record that exists in both places with different content or
author is shown as a diff of the fields that differ and you answer `l` (keep yours) or `c` (take the
cloud's) — or `--take local|cloud` for all, which a script needs, since no answer stops the pull before
anything is written. A record you keep is marked in `~/.flmnt/sync-marks.json` and your next push
replaces its cloud copy (`replace`, admitted for records of your own identities); the mark is then
cleared. Counts are printed per stream: appended, skipped, overwritten, kept, replaced.

**Connecting to a teammate's harness.** When the approval page belongs to someone else's flmnt (behind
their tunnel), the host's `approve` would otherwise sign you in as the HOST. Before they approve, run
`flmnt local claim <code> --server-url <their PUBLIC_URL>` on your machine: the CLI asks their edge
what the code is waiting for (`GET /local/claim?code=`), confirms (`Claim Claude (…) on <url> for your
identity local_…? [y/N]`), and posts your own identity — the sub `flmnt login --local` recorded here
and the client id derived for the client's name — to `POST /local/claim`. Their `approve` then shows
"for sub local_<you> (claimed from another machine)" and keeps your identity, so your writes are
yours (QRM-ADR-016 §5a, SPEC-LOCAL-007). One claim per code; `LOCAL_CLAIM_ALREADY_MADE` and the
other refusals are surfaced verbatim. Tell the host your code; the code is how they verify it is you.

`flmnt login --local` also obtains the CLI's own harness token once the login is recorded: when the
session's server advertises a device grant in its OAuth discovery, the CLI runs the device grant as
`flmnt-cli` and approves its own user code over the admin listener — nobody is sent to a browser —
then stores the token under that server URL as any login does and records the OAuth endpoints in the
session. Otherwise the login is recorded as before and the token step says it was skipped and why.

**Credentials.** Every command — `brief`, `gate`, `derive`, `record-*`, `workspace`, `sync`'s local
side — authenticates from the keychain token stored under the server URL it resolves: the one
`flmnt login` stores for a hosted flmnt, the one `flmnt login --local` stores for the harness. There
is no other source: a server with no stored token is refused as ``not logged in to <server>: run
`flmnt login --local --server-url <server>` `` (or `flmnt login` for a hosted one), nothing is
substituted, and `flmnt health` reports the local token — who it is for, or that same refusal. The
`.mcp.json` entry `flmnt setup` writes is url-only in both environments: `{"type":"http","url":"<server>"}`,
no headers, and the client runs the OAuth flow the MCP advertises.

A local server has no OAuth discovery to advertise its router, so the checkout declares it in
`.flmnt.json` as `"local_graphql_endpoint": "http://localhost:4000/graphql"`; on a loopback server
that is read before the `<server>/graphql` fallback (`--graphql-endpoint` / `FILAMENT_GRAPHQL_ENDPOINT`
still win), and `flmnt setup` preserves the field. `flmnt setup` always rewrites the `flmnt` entry of
`.mcp.json` to what it is asked for (other servers stay as they are); `pnpm env:local` in the checkout
restores the local entry.

### Which workspace a command uses

`brief`, `derive`, `gate` and `record-*` resolve the project in this order, first match wins:

1. an explicit `--project <name|id>` on the command
2. `project_id` in the repo's `.flmnt.json` — written by `flmnt setup --project` (a repo still holding
   the former `.quorum.json` alone is read from it, and `flmnt whoami` says to rename it)
3. the active workspace (`flmnt workspace use`)

Step 3 is the fallback of last resort, not the norm. A repo that reaches it has no identity of its
own, so a per-machine setting decides where its memory goes — which is why `setup` now refuses to
configure a repo without naming a workspace.

## Commands

```sh
flmnt help                       # list all commands
flmnt <command> -h               # detailed help for a command

# Authentication
flmnt login                      # authenticate via OAuth2 (browser PKCE, or --device for headless); becomes THE login
flmnt login --local              # record the local login (derived from this machine + OS user) a local flmnt mints for; becomes THE login,
                                 # and obtains the CLI's own token from the local server (--admin-url, default http://127.0.0.1:4012)
flmnt logout                     # sign out: clear the session and every stored token; the local identity stays
flmnt logout --local             # end only a local session
flmnt whoami                     # show your identity (from the login; no --server-url needed) and which workspace is in force here
flmnt whoami --json [--client <tool>]   # the identity as one JSON object (exit 1 when not logged in)

# Local harness
flmnt local approve <code> [--yes] [--admin-url <url>]   # approve a client's pending sign-in on the local broker for the local login
flmnt local claim <code> --server-url <url> [--yes]      # claim a teammate's pending code for this machine's identity, before they approve

# Workspaces
flmnt workspace list             # list workspaces you own or are a member of
flmnt workspace create <name>    # create a workspace and make it active
flmnt workspace use <name|id>    # set the active workspace (sent as X-Workspace-Id)
flmnt workspace rename <name|id> <new-name>       # rename a workspace you own
flmnt workspace delete <name|id> --yes            # delete a workspace you own (refuses without --yes)
flmnt workspace members <name|id>                 # list members of a workspace
flmnt workspace add-member <name|id> @username    # add a member to a workspace you own
flmnt workspace remove-member <name|id> @username # remove a member from a workspace you own

# MCP / Claude Code integration
flmnt setup --server-url <url> --project <name|id>
                                 # install the automation kit: .mcp.json + full lifecycle hook map
                                 # + .claude/commands/flmnt-* slash commands + tool permissions (idempotent)
                                 # --project is REQUIRED and pins THIS repo to that workspace
flmnt proxy                      # run the local MCP proxy (injects Authorization: Bearer)
flmnt mcp auth-header            # print MCP auth headers as JSON for the .mcp.json headersHelper

# Data
flmnt sync push                  # sync local data up to the remote workspace as your cloud account; your local identity is linked to it, records keep their author
flmnt link                       # link this machine's local identity (and its history) to the hosted account you are logged into, pushing nothing
flmnt sync pull [--take local|cloud]   # sync remote data down to local; each record that differs in both places is a diff you decide
                                 # the local side authenticates from the token `flmnt login --local` stores, minted again from your identity when a hosted login swept it

# Memory (continuity loop + deterministic writes for hooks / CI)
flmnt brief                      # SessionStart: inject latest keyframe + recent decisions + directions/questions + mistakes
flmnt derive --hook              # Stop: derive prompts/keyframes/mistakes from transcript + git
flmnt derive --intent            # how captured prompts classify, and the openers the lexicon missed
flmnt record-metric --name <n> --value <v>   # write an operational metric to {project}::metrics
flmnt record-metric --hook       # PostToolUse: emit a CI metric for the command that just ran (stdin)
flmnt record-plan --content <p>  # write a multi-step plan to {project}::plan
flmnt record-supersession --content <c> --supersedes <id>  # replace a decision (SUPERSEDED_BY edge)
flmnt record-attestation --kind <k> --note <n>             # ContextAttestation metric

# Doctrine as markdown (generated — never hand-maintained)
flmnt corpus --out training       # render the workspace's DOC-NODE documents from its domain stream
flmnt corpus --project platform --out training   # …from another workspace you can reach
flmnt corpus --hook --project platform            # SessionStart: refresh silently, never fail a start

# Utilities
flmnt health                     # probe the configured server's /health (local or hosted) and, locally, the stored local token
flmnt gate                       # keyframe-recency check for the UserPromptSubmit hook
flmnt dashboard                  # open the dashboard in a browser
flmnt version                    # print the version
```

## License

[Apache-2.0](./LICENSE). The flmnt CLI is open source; the flmnt platform is proprietary.
