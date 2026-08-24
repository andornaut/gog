package version

import (
	"os"
	"runtime/debug"
	"strings"
	"testing"
)

// The stamp is the only thing that puts a release number on a binary, and a -X
// naming a symbol that is not there does nothing and exits 0: the release would
// publish a binary reporting the "dev" below, and nothing would say so until
// someone ran it. This holds the release config to the symbol this package
// declares, reading the module path rather than spelling it twice.
func TestTheReleaseConfigStampsThisSymbol(t *testing.T) {
	want := "-X " + modulePath(t) + "/internal/version.Version={{.Version}}"
	config, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatalf("reading the release config: %v", err)
	}
	if !strings.Contains(string(config), want) {
		t.Errorf("the release config does not stamp this package.\nwant ldflags containing: %s", want)
	}
}

// A stamped binary reports its stamp. An unstamped one may claim only a
// version the module system recorded for a build that came from the module
// cache: a build from a working tree records VCS settings, and is a local build
// whatever tag the tree is sitting on.
func TestReported(t *testing.T) {
	fromCache := &debug.BuildInfo{Main: debug.Module{Version: "v1.3.4"}}
	fromWorkingTree := &debug.BuildInfo{
		Main:     debug.Module{Version: "v1.3.4"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "2246f658d5ff"}},
	}

	tests := []struct {
		name    string
		stamped string
		info    *debug.BuildInfo
		want    string
	}{
		{name: "the stamp is kept", stamped: "1.2.3", info: fromCache, want: "1.2.3"},
		{name: "a module cache build reports what was recorded", stamped: "dev", info: fromCache, want: "1.3.4"},
		{name: "a working tree build stays dev", stamped: "dev", info: fromWorkingTree, want: "dev"},
		{
			name:    "a recorded version that names no release stays dev",
			stamped: "dev",
			info:    &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			want:    "dev",
		},
		{name: "no build info at all stays dev", stamped: "dev", want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reported(tt.stamped, tt.info); got != tt.want {
				t.Errorf("reported(%q) = %q, want %q", tt.stamped, got, tt.want)
			}
		})
	}
}

// releaseVersion is what decides whether an unstamped binary may claim a
// version, and what it claims. It has to reject everything the module system
// records that is not a release, and to spell a release the way the stamp does:
// the two halves reporting one release differently would make the version
// depend on how the binary was installed.
func TestReleaseVersionAcceptsOnlyAReleaseAndDropsThePrefix(t *testing.T) {
	for _, tc := range []struct {
		recorded string
		want     string
	}{
		{"v1.3.4", "1.3.4"},
		{"v0.0.0", "0.0.0"},
		{"v10.20.30", "10.20.30"},
		// A pseudo-version: `go install <module>@main` records one of these.
		{"v1.3.5-0.20260819015529-2246f658d5ff", ""},
		// A build off a modified tree.
		{"v1.3.4+dirty", ""},
		// Built outside the module system.
		{"(devel)", ""},
		{"", ""},
		{"dev", ""},
		{"1.3.4", ""},
		{"v1.3", ""},
		{"v1.3.4.5", ""},
		{"v1.3.x", ""},
		{"v1..4", ""},
		{"v2.0.0+incompatible", ""},
	} {
		if got := releaseVersion(tc.recorded); got != tc.want {
			t.Errorf("releaseVersion(%q) = %q, want %q", tc.recorded, got, tc.want)
		}
	}
}

func modulePath(t *testing.T) string {
	t.Helper()
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	for line := range strings.SplitSeq(string(mod), "\n") {
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	t.Fatal("go.mod names no module")
	return ""
}
