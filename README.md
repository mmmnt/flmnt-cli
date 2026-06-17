# flmnt

The developer CLI for the [flmnt](https://flmnt.dev) event-stream memory platform — authentication, workspace management, and the MCP auth helper for connecting clients like Claude Code to a live flmnt MCP.

## Install

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
flmnt login                      # authenticate (OAuth2 device or browser)
flmnt workspace list             # list workspaces you own or share
flmnt workspace use <name|id>    # set the active workspace
flmnt mcp auth-header            # print MCP auth headers for .mcp.json headersHelper
```

Wire the CLI into an MCP client via `.mcp.json`:

```json
{
  "headersHelper": "flmnt mcp auth-header --server-url https://mcp.<env>.flmnt.dev"
}
```

## License

[Apache-2.0](./LICENSE). The flmnt CLI is open source; the flmnt platform is proprietary.
