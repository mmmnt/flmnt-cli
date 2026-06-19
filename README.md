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
flmnt login                              # OAuth2 — browser PKCE (or --device for headless)
flmnt workspace use <name|id>            # set the active workspace
flmnt setup --server-url <flmnt-url>     # write .mcp.json + the keyframe-gate hook
```

`flmnt setup` writes a project-local `.mcp.json` pointing at the local proxy plus a
`.claude/settings.local.json` UserPromptSubmit hook, then `flmnt proxy` injects your
bearer token on outbound MCP requests — so Claude Code talks to a live, authenticated
flmnt MCP without you handling tokens by hand. `setup` is idempotent.

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
flmnt workspace use <name|id>    # set the active workspace (sent as X-Workspace-Id)
flmnt workspace rename <name>    # rename a workspace you own
flmnt workspace delete <name|id> # delete a workspace you own
flmnt workspace members          # list members of a workspace
flmnt workspace add-member       # add a member to a workspace you own
flmnt workspace remove-member    # remove a member from a workspace you own

# MCP / Claude Code integration
flmnt setup --server-url <url>   # write .mcp.json + keyframe-gate hook (idempotent)
flmnt proxy                      # run the local MCP proxy (injects Authorization: Bearer)
flmnt mcp auth-header            # print MCP auth headers as JSON for the .mcp.json headersHelper

# Data
flmnt sync push                  # sync local Quorum data up to the remote workspace
flmnt sync pull                  # sync remote workspace data down to local

# Utilities
flmnt health                     # check health of Core, Engine, and proxy services
flmnt gate                       # keyframe-recency check for the UserPromptSubmit hook
flmnt dashboard                  # open the dashboard in a browser
flmnt version                    # print the version
```

## License

[Apache-2.0](./LICENSE). The flmnt CLI is open source; the flmnt platform is proprietary.
