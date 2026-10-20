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

### Local harness (no `flmnt login` needed)

A checkout can run against a local flmnt instead of the hosted one. The CLI tells the two apart by
the server URL it resolves (`--server-url` / `QUORUM_SERVER_URL` / `.quorum.json` / the login config):
a loopback host — `http://localhost:8000/mcp` — is local, anything else is hosted.

- **Hosted**: credentials come from the keychain `flmnt login` filled, exactly as above.
- **Local**: credentials come from the checkout's own headers-helper — the `headersHelper` on the
  `flmnt` entry in `.mcp.json`, the same command Claude Code runs for that server. The CLI runs it
  with `BROKER_CLIENT_ID=flmnt-cli` in its environment, so what it writes is attributed to the CLI
  (the entry's own `--client <id>` argument is Claude Code's declaration and is not passed along).
  The helper prints `{"Authorization":"Bearer …","X-Workspace-Id":"…"}`; the workspace is the
  session's project when the repo has no pin, and `sync`'s `--local-workspace` default. A helper that
  refuses (nobody logged in, no client named) is "not configured": nothing is substituted, and
  `flmnt health` shows its reason.
- `--auth-cmd <command>` (any command) names a helper explicitly and is run verbatim; without one
  and without a `.mcp.json` entry, the repo-relative `bash scripts/mcp-auth-header.sh` runs only inside
  a configured repo (a `.quorum.json`).
- A local server has no OAuth discovery, so point `FILAMENT_GRAPHQL_ENDPOINT` (or
  `--graphql-endpoint`) at its router when the GraphQL endpoint is not advertised.

`brief`, `gate`, `derive`, `record-*`, `whoami`, `workspace` and `sync`'s local side all take this path.
`flmnt setup` leaves a loopback `flmnt` entry that carries a `headersHelper` as it is.

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
flmnt logout                     # sign out and revoke local credentials
flmnt whoami                     # show your identity and which workspace is in force here

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
