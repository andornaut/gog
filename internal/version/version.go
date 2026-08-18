// Package version holds the one version string the binary reports. Its own
// package so that every program in the module reaches the same value without
// importing the one that happens to declare it.
package version

// Version is the build version reported by the version command. A var rather
// than a const because the linker stamps it: -X takes a variable and silently
// does nothing to a constant. The ldflags in .goreleaser.yaml set it for a
// tagged build, and every other build, the rolling dev archive included,
// reports "dev".
var Version = "dev"
