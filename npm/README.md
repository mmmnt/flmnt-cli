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
scoop install flmnt                          # Scoop (Windows)
go install github.com/mmmnt/flmnt-cli@latest # Go
```

## Quickstart

Authenticate, pick a workspace, and wire Claude Code to your flmnt MCP:

```sh
flmnt login                              # OAuth2 — browser PKCE (or --device for headless)
flmnt workspace use <name|id>            # set the active workspace
flmnt setup --server-url <flmnt-url>     # write .mcp.json + the automation kit
```

`flmnt setup` writes a project-local `.mcp.json` pointing at the local proxy plus a `.claude/` hook map and slash commands; `flmnt proxy` then injects your bearer token on outbound MCP requests — so Claude Code talks to a live, authenticated flmnt MCP without you handling tokens by hand. `setup` is idempotent.

## Commands

```sh
flmnt help                       # list all commands
flmnt <command> -h               # detailed help for a command

# Authentication
flmnt login                      # authenticate via OAuth2 (browser PKCE, or --device for headless)
flmnt logout                     # sign out and revoke local credentials
flmnt whoami                     # show the active identity and workspace

# Workspaces
flmnt workspace list             # list workspaces you own or are a member of
flmnt workspace create <name>    # create a workspace and make it active
flmnt workspace use <name|id>    # set the active workspace
flmnt workspace members          # list members of a workspace
flmnt workspace add-member       # add a member to a workspace you own
flmnt workspace remove-member    # remove a member from a workspace you own

# MCP / Claude Code integration
flmnt setup --server-url <url>   # install the automation kit: .mcp.json + lifecycle hooks
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

# Utilities
flmnt health                     # check health of Core, Engine, and proxy services
flmnt gate                       # keyframe-recency check for the UserPromptSubmit hook
flmnt dashboard                  # open the dashboard in a browser
flmnt version                    # print the version
```

## License

[Apache-2.0](./LICENSE). The flmnt CLI is open source; the flmnt platform is proprietary.
