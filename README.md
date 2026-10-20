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
(the loopback server, the derived sub). Either login replaces the other entirely — every stored token
goes, the local login goes — and `flmnt logout` leaves nothing at all: the session, every token for
every server, the local login, the file keyring of older builds. Every command takes its upstream from
the session, so a logged-in person never passes `--server-url`; the chain is `--server-url` /
`QUORUM_SERVER_URL` / **the login** / `.quorum.json` (the repo answers only when nothing is logged
in). A loopback host — `http://localhost:8000/mcp` — is local, anything else is hosted. The hosted
path is unchanged: credentials come from the keychain `flmnt login` filled.

**Identity.** A local flmnt mints credentials for a declared person, never for a default:

```sh
flmnt login --local                           # once per machine and OS user — no input
flmnt whoami --json --client flmnt-cli        # what a local headers-helper asks
flmnt logout --local                          # end the local login
```

`login --local` reads an identifying marker the machine already carries (macOS `IOPlatformUUID`,
Linux `/etc/machine-id`, Windows `MachineGuid`) and the OS user, and derives, deterministically:
`sub = "local_" + UUIDv5(c0a3f0d2-5b1e-4d7a-9c3e-2f6b8a1d4e70, marker + "\n" + osUser)` and
`client_id = "local_" + UUIDv5(sub's UUID, <tool name>)`. The same machine and user give the same sub
forever, a tool name the same client id forever, and two people or two machines never collide; the
`local_` prefix lets the hosted side recognise a local identity without a lookup. Only the digest is
kept — `~/.filament/local-identity.json` (0600) holds `{sub, created_at}`, never the marker — and that
file is what makes the login explicit. A machine with no readable marker is refused; nothing is
generated at random and nothing is reused from the hosted login. `login --local` takes the loopback
server from `--server-url` / `QUORUM_SERVER_URL` when one is named (a hosted URL is refused), else
`http://localhost:8000`, and records it as the session's server. `whoami --json` prints one object
`{"sub","workspace","client_id"?}`: from the local login when that is the session, from the stored
OAuth login when a hosted one is (`client_id` is then the registered OAuth client); `client_id` only
appears with `--client`. Not logged in (either kind): nothing on stdout, one reason line on stderr, exit 1. Plain
`whoami` says `local identity <sub>` for the local login.

**Credentials.** On a loopback server every command — `brief`, `gate`, `derive`, `record-*`, `whoami`,
`workspace`, `sync`'s local side — authenticates through the checkout's own headers-helper: the
`headersHelper` on the `flmnt` entry in `.mcp.json`, the same command Claude Code runs for that
server. The CLI runs it with `--client flmnt-cli` appended (the helper takes the last `--client`, so
the entry's own declaration for Claude Code does not apply), so the helper mints for the client id
derived for the CLI and its writes are attributed to it. The helper prints `{"Authorization":"Bearer …","X-Workspace-Id":"…"}`;
the workspace is the session's project when the repo has no pin, and `sync`'s `--local-workspace`
default. A helper that refuses is "not configured": its reason is surfaced verbatim, nothing is
substituted, and `flmnt health` shows it. `--auth-cmd <command>` names a helper explicitly; without
one and without a `.mcp.json` entry, the repo-relative `bash scripts/mcp-auth-header.sh` runs only
inside a configured repo (a `.quorum.json`). On a hosted server the helper is consulted only when no
login is stored.

A local server has no OAuth discovery to advertise its router, so the checkout declares it in
`.quorum.json` as `"local_graphql_endpoint": "http://localhost:4000/graphql"`; on a loopback server
that is read before the `<server>/graphql` fallback (`--graphql-endpoint` / `FILAMENT_GRAPHQL_ENDPOINT`
still win), and `flmnt setup` preserves the field. `flmnt setup` always rewrites the `flmnt` entry of
`.mcp.json` to what it is asked for (other servers stay as they are); `pnpm env:local` in the checkout
restores the local entry.

### Which workspace a command uses

`brief`, `derive`, `gate` and `record-*` resolve the project in this order, first match wins:

1. an explicit `--project <name|id>` on the command
2. `project_id` in the repo's `.quorum.json` — written by `flmnt setup --project`
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
flmnt login --local              # record the local login (derived from this machine + OS user) a local flmnt mints for; becomes THE login
flmnt logout                     # sign out: clear the session, every stored token, and the local login
flmnt logout --local             # remove only the local login
flmnt whoami                     # show your identity (from the login; no --server-url needed) and which workspace is in force here
flmnt whoami --json [--client <tool>]   # the identity as JSON for a local headers-helper (exit 1 when not logged in)

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
flmnt sync push                  # sync local Quorum data up to the remote workspace
flmnt sync pull                  # sync remote workspace data down to local
                                 # the local side authenticates through the checkout's headers-helper

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
flmnt health                     # probe the configured server's /health (local or hosted) and, locally, the headers-helper
flmnt gate                       # keyframe-recency check for the UserPromptSubmit hook
flmnt dashboard                  # open the dashboard in a browser
flmnt version                    # print the version
```

## License

[Apache-2.0](./LICENSE). The flmnt CLI is open source; the flmnt platform is proprietary.
