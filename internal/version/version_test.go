package version

import (
	"os"
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

// A build made outside a release must not claim to be one. The hazard is a
// hand-edited number left behind after a release, which reports the previous
// version from every working tree until someone edits it again.
func TestTheUnstampedDefaultIsDev(t *testing.T) {
	if Version != "dev" {
		t.Errorf("the unstamped default is %q, want %q", Version, "dev")
	}
}

// isRelease is what decides whether an unstamped binary may claim a version, so
// it has to reject everything the module system records that is not one.
func TestIsReleaseAcceptsOnlyAReleasedVersion(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"v1.3.4", true},
		{"v0.0.0", true},
		{"v10.20.30", true},
		// A pseudo-version: `go install <module>@main` records one of these.
		{"v1.3.5-0.20260819015529-2246f658d5ff", false},
		// A build off a modified tree.
		{"v1.3.4+dirty", false},
		// Built outside the module system.
		{"(devel)", false},
		{"", false},
		{"dev", false},
		{"1.3.4", false},
		{"v1.3", false},
		{"v1.3.4.5", false},
		{"v1.3.x", false},
		{"v1..4", false},
	} {
		if got := isRelease(tc.version); got != tc.want {
			t.Errorf("isRelease(%q) = %v, want %v", tc.version, got, tc.want)
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
