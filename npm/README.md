# @mmmnt/flmnt

The developer CLI for the [flmnt](https://flmnt.ai) event-stream memory platform — authentication, workspace management, and the MCP auth helper that wires clients like Claude Code to a live, authenticated flmnt MCP.

This package is a thin installer: on `postinstall` it downloads the platform-native `flmnt` binary for your OS/architecture from the matching [GitHub release](https://github.com/mmmnt/flmnt-cli/releases), verifies it against the release checksums, and exposes it as the `flmnt` command. No Go toolchain required.

## Install

```sh
npm install -g @mmmnt/flmnt
```

Requires Node.js ≥ 18. Supported platforms: macOS, Linux, and Windows on x64 and arm64.

Prefer a native package manager? The same CLI ships through several channels:

```sh
brew install mmmnt/tap/flmnt                 # Homebrew (macOS/Linux)
scoop bucket add flmnt https://github.com/mmmnt/scoop-bucket   # Scoop (Windows): add the bucket first
scoop install flmnt
go install github.com/mmmnt/flmnt-cli@latest # Go
```

## Quickstart

Authenticate, pick a workspace, and wire Claude Code to your flmnt MCP:

```sh
flmnt login                              # OAuth2 — browser PKCE (or --device for headless)
flmnt setup --server-url <flmnt-url> --project <name>   # wire this repo to a workspace
```

`--project` names the workspace **this repo** records into, by name — you do not need its id. It is required, and it pins the repo: every hook targets that workspace regardless of what the CLI is pointed at elsewhere.

`flmnt setup` writes a project-local `.mcp.json` pointing at the local proxy plus a `.claude/` hook map and slash commands; `flmnt proxy` then injects your bearer token on outbound MCP requests — so Claude Code talks to a live, authenticated flmnt MCP without you handling tokens by hand. `setup` is idempotent, and it always rewrites the `flmnt` entry of `.mcp.json` to what it is asked for; other servers in that file stay as they are.

### Local flmnt (the compose harness)

A checkout can run against a local flmnt instead of the hosted one. The CLI tells the two apart by the server URL it resolves (`--server-url` / `QUORUM_SERVER_URL` / `.quorum.json` / the login config): a loopback host such as `http://localhost:8000/mcp` is local, anything else is hosted. The hosted path is unchanged — credentials come from the keychain `flmnt login` filled.

A local flmnt mints credentials for a declared person, never for a default, and asks for nothing:

```sh
flmnt login --local                      # once per machine and OS user — no input, no email
flmnt whoami --json --client flmnt-cli   # {"sub","workspace","client_id"} — the identity as one JSON object
flmnt logout --local                     # end the local login
```

`login --local` derives the identity from a marker the machine already carries (macOS `IOPlatformUUID`, Linux `/etc/machine-id`, Windows `MachineGuid`) and the OS user: `sub = "local_" + UUIDv5(marker + OS user)`, and per tool `client_id = "local_" + UUIDv5(sub, tool)`. Only the digest is kept (`~/.filament/local-identity.json`); the same machine and user give the same sub forever, and two people or two machines never collide. No login, on either side, prints nothing on stdout, one reason on stderr, and exits 1.

Every command — `brief`, `gate`, `derive`, `record-*`, `workspace`, `sync`'s local side — authenticates from the keychain token stored under the server URL it resolves: `flmnt login` stores it for a hosted flmnt, `flmnt login --local` for the harness (the device grant as `flmnt-cli`, approved by the CLI itself over the broker's loopback admin listener). A server with no stored token is refused as ``not logged in to <server>: run `flmnt login --local --server-url <server>` `` (or `flmnt login` for a hosted one); nothing is substituted. The `.mcp.json` entry `flmnt setup` writes is url-only in both environments. The GraphQL endpoint of a local server comes from `local_graphql_endpoint` in the checkout's `.quorum.json` (`FILAMENT_GRAPHQL_ENDPOINT` overrides it); `flmnt setup` carries that field through. `flmnt health` names the environment it resolved, probes that server's `/health`, reports the local token on a loopback server, and probes Core/Engine/proxy only when `CORE_URL` / `ENGINE_URL` / `QUORUM_PROXY_PORT` are set.

### Which workspace a command uses

`brief`, `derive`, `gate` and `record-*` resolve the project in this order, first match wins:

1. an explicit `--project <name|id>` on the command
2. `project_id` in the repo's `.quorum.json` — written by `flmnt setup --project`
3. the active workspace (`flmnt workspace use`)

Step 3 is a fallback of last resort, not the norm: a repo that reaches it has no identity of its own, so a per-machine setting decides where its memory goes. `flmnt workspace use` remains a convenience for ad-hoc commands and does not decide where a configured repo records.

## Commands

```sh
flmnt help                       # list all commands
flmnt <command> -h               # detailed help for a command

# Authentication
flmnt login                      # authenticate via OAuth2 (browser PKCE, or --device for headless)
flmnt login --local              # a local flmnt: derive this machine's identity, no input
flmnt logout                     # sign out and revoke local credentials
flmnt logout --local             # end the local login
flmnt whoami                     # show your identity and which workspace is in force here
flmnt whoami --json [--client <tool>]   # {"sub","workspace","client_id"?} as one JSON object

# Workspaces
flmnt workspace list             # list workspaces you own or are a member of
flmnt workspace create <name>    # create a workspace and make it active
flmnt workspace use <name|id>    # set the active workspace
flmnt workspace members          # list members of a workspace
flmnt workspace add-member       # add a member to a workspace you own
flmnt workspace remove-member    # remove a member from a workspace you own

# MCP / Claude Code integration
flmnt setup --server-url <url> --project <name|id>
                                 # install the automation kit: .mcp.json + lifecycle hooks
                                 # --project is REQUIRED and pins THIS repo to that workspace
                                 # + .claude/commands/flmnt-* slash commands (idempotent)
flmnt proxy                      # run the local MCP proxy (injects Authorization: Bearer)
flmnt mcp auth-header            # print MCP auth headers as JSON for .mcp.json

# Data
flmnt sync push                  # sync local Quorum data up to the remote workspace
flmnt sync pull                  # sync remote workspace data down to local

# Memory (continuity loop + deterministic writes for hooks / CI)
flmnt brief                      # SessionStart: inject latest keyframe + recent decisions + mistakes
flmnt derive --hook              # Stop: derive decisions/keyframes/mistakes from transcript + git
flmnt record-metric --name <n> --value <v>   # write an operational metric
flmnt record-plan --content <p>              # write a multi-step plan
flmnt record-supersession --content <c> --supersedes <id>  # replace a decision

# Doctrine as markdown (generated — never hand-maintained)
flmnt corpus --out training                   # render the DOC-NODE documents in the domain stream
flmnt corpus --hook --project platform        # SessionStart refresh; wired by setup --corpus-project

# Utilities
flmnt health                     # the resolved environment + the server's /health (Core/Engine/proxy when declared)
flmnt gate                       # keyframe-recency check for the UserPromptSubmit hook
flmnt dashboard                  # open the dashboard in a browser
flmnt version                    # print the version
```

## License

[Apache-2.0](./LICENSE). The flmnt CLI is open source; the flmnt platform is proprietary.
