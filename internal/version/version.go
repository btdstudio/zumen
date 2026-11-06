// Package version reports the zumen version.
package version

import "runtime/debug"

// String returns the zumen version with a leading "v" (e.g. "v0.1.0"). A binary
// installed via `go install pkg@version` reports the module version from build
// info (the git tag), so releases never need a manual version bump. Local
// `go build`/`go run` builds report "dev".
func String() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return fromBuildInfo(info)
}

// fromBuildInfo extracts the version from module build info, returning "dev"
// for non-module or devel builds.
func fromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return "dev"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "dev"
	}
	return v
}
