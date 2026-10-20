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

A checkout can run against a local flmnt instead of the hosted one. The CLI tells the two apart by
the server URL it resolves (`--server-url` / `QUORUM_SERVER_URL` / `.quorum.json` / the login config):
a loopback host — `http://localhost:8000/mcp` — is local, anything else is hosted. The hosted path is
unchanged: credentials come from the keychain `flmnt login` filled.

**Identity.** A local flmnt mints credentials for a declared person, never for a default:

```sh
flmnt login --local --email you@example.com   # once per machine
flmnt whoami --json --client flmnt-cli        # what a local headers-helper asks
```

`login --local` records the email plus an identifying marker the machine already carries (macOS
`IOPlatformUUID`, Linux `/etc/machine-id`, Windows `MachineGuid`) in `~/.filament/local-identity.json`
(0600), and refuses without an email or a readable marker. From those two, deterministically:
`sub = UUIDv5(c0a3f0d2-5b1e-4d7a-9c3e-2f6b8a1d4e70, marker + "\n" + email)` and
`client_id = UUIDv5(sub, <tool name>)` — the same machine and email give the same sub forever, a tool
name the same client id forever, and two people or two machines never collide. Nothing is reused from
the hosted login. `whoami --json` prints one object `{"sub","email","username"?,"workspace","client_id"?}`:
on a loopback server from the local login, elsewhere from the stored OAuth login (`client_id` is then
the registered OAuth client); `username` is omitted when unknown and `client_id` only appears with
`--client`. Not logged in (either kind): nothing on stdout, one reason line on stderr, exit 1.

**Credentials.** On a loopback server every command — `brief`, `gate`, `derive`, `record-*`, `whoami`,
`workspace`, `sync`'s local side — authenticates through the checkout's own headers-helper: the
`headersHelper` on the `flmnt` entry in `.mcp.json`, the same command Claude Code runs for that
server. The CLI runs it with `--client flmnt-cli` appended (the entry's own `--client <tool>` is Claude
Code's declaration and is dropped), so the helper mints for the client id derived for the CLI and its
writes are attributed to it. The helper prints `{"Authorization":"Bearer …","X-Workspace-Id":"…"}`;
the workspace is the session's project when the repo has no pin, and `sync`'s `--local-workspace`
default. A helper that refuses is "not configured": its reason is surfaced verbatim, nothing is
substituted, and `flmnt health` shows it. `--auth-cmd <command>` names a helper explicitly; without
one and without a `.mcp.json` entry, the repo-relative `bash scripts/mcp-auth-header.sh` runs only
inside a configured repo (a `.quorum.json`). On a hosted server the helper is consulted only when no
login is stored.

A local server has no OAuth discovery, so point `FILAMENT_GRAPHQL_ENDPOINT` (or `--graphql-endpoint`)
at its router when the GraphQL endpoint is not advertised. `flmnt setup` leaves a loopback `flmnt`
entry that carries a `headersHelper` as it is.

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
flmnt login                      # authenticate via OAuth2 (browser PKCE, or --device for headless)
flmnt login --local --email <e>  # record the local login a local flmnt mints credentials for
flmnt logout                     # sign out and revoke local credentials
flmnt whoami                     # show your identity and which workspace is in force here
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
