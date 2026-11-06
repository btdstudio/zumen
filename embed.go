// Package zumen embeds the canonical planning ICM scaffold so the zumen binary
// is self-contained: no source checkout is required at runtime. `zumen init`
// copies this tree out; `zumen update` refreshes managed files against it.
//
// `all:` includes dot-files (.gitignore, .gitkeep), which a plain go:embed
// pattern would silently ignore.
package zumen

import "embed"

// ScaffoldFS is the embedded planning ICM scaffold tree.
//
//go:embed all:scaffold
var ScaffoldFS embed.FS
