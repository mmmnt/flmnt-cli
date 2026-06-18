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

## Usage

```sh
flmnt help                       # for a list of flmnt commands
flmnt <command> -h               # for detailed help on a command
flmnt login                      # authenticate (OAuth2 device or browser)
flmnt workspace list             # list workspaces you own or share
flmnt workspace use <name|id>    # set the active workspace
flmnt mcp auth-header            # print MCP auth headers for .mcp.json headersHelper
```

## License

[Apache-2.0](./LICENSE). The flmnt CLI is open source; the flmnt platform is proprietary.
