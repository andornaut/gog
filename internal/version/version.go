// Package version holds the one version string the binary reports. Its own
// package so that every program in the module reaches the same value without
// importing the one that happens to declare it.
package version

import (
	"runtime/debug"
	"strings"
)

// Version is the build version reported by the version command. A var rather
// than a const because the linker stamps it: -X takes a variable and silently
// does nothing to a constant. The ldflags in .goreleaser.yaml set it for a
// tagged build, and init below decides what an unstamped one reports.
var Version = "dev"

// A binary the linker did not stamp can still know what it was built from:
// `go install <module>@v1.2.3` records the version and records no VCS settings,
// having built from the module cache rather than from a checkout. A build made
// from a working tree records vcs.revision, and is a local build whatever the
// tree is sitting on, so it keeps "dev" rather than claiming the tag under it.
func init() {
	if Version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, setting := range info.Settings {
		if strings.HasPrefix(setting.Key, "vcs") {
			return
		}
	}
	if v := releaseVersion(info.Main.Version); v != "" {
		Version = v
	}
}

// releaseVersion returns what to report for a version the module system
// recorded, or "" where that version does not name a release. A pseudo-version
// carries a timestamp and a revision, a build off a modified tree carries
// +dirty, and a module built outside the module system reports "(devel)"; none
// of those names a release, and a binary reporting one would be claiming to be
// something it is not.
//
// The leading v is dropped so that both halves spell one release the same way:
// the stamp is goreleaser's {{.Version}}, which is the tag without it, and the
// module system records the tag with it. Reporting 1.2.3 from the published
// archive and v1.2.3 from `go install` would make the version depend on how the
// binary arrived.
func releaseVersion(v string) string {
	digits, found := strings.CutPrefix(v, "v")
	if !found {
		return ""
	}
	parts := strings.Split(digits, ".")
	if len(parts) != 3 {
		return ""
	}
	for _, part := range parts {
		if part == "" || strings.TrimLeft(part, "0123456789") != "" {
			return ""
		}
	}
	return digits
}
