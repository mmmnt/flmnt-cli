package setup

import "embed"

// assets bundles the flmnt automation kit written by `flmnt setup`: the slash-command catalog
// (.claude/commands/*.md) and the generic hook scripts (.claude/flmnt-hooks/*.sh). Embedding keeps
// the kit shipping inside the single CLI binary — no network fetch, versioned with the CLI.
//
//go:embed assets/commands/*.md assets/hooks/*.sh
var assets embed.FS
