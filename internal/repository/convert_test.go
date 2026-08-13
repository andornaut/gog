package repository

import (
	"testing"
)

const (
	testHomeDir  = "/home/testuser"
	testRepoPath = testHomeDir + "/.local/share/gog/testrepo"
)

// setHomeDir points the package at a home directory for the duration of the
// test
func setHomeDir(t *testing.T, home string) {
	t.Helper()
	original := SetHomeDirForTest(home)
	t.Cleanup(func() { SetHomeDirForTest(original) })
}

// Only $HOME is expanded. Expanding whatever a component names would let the
// environment decide where a repository writes.
func TestToExternalPath(t *testing.T) {
	setHomeDir(t, testHomeDir)

	tests := []struct {
		name string
		p    string
		want string
	}{
		{name: "$HOME is expanded", p: testRepoPath + "/$HOME/.bashrc", want: "/home/testuser/.bashrc"},
		{name: "$PATH is not", p: testRepoPath + "/$PATH/file", want: "/$PATH/file"},
		{name: "$USER is not", p: testRepoPath + "/$USER/.config", want: "/$USER/.config"},
		{name: "a name that merely begins with $HOME is not", p: testRepoPath + "/$HOMEWORK/file", want: "/$HOMEWORK/file"},
		{name: "anything else belongs to the filesystem root", p: testRepoPath + "/etc/hosts", want: "/etc/hosts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToExternalPath(testRepoPath, tt.p); got != tt.want {
				t.Errorf("ToExternalPath(%q) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

// A path under the home directory is stored by the portable $HOME component,
// and the home directory is matched on a path boundary so that a sibling of it
// is not read as being within it
func TestToInternalPath(t *testing.T) {
	setHomeDir(t, testHomeDir)

	tests := []struct {
		name string
		p    string
		want string
	}{
		{name: "within home", p: "/home/testuser/.bashrc", want: testRepoPath + "/$HOME/.bashrc"},
		{name: "a sibling of home", p: "/home/testuserother/.bashrc", want: testRepoPath + "/home/testuserother/.bashrc"},
		{name: "outside home", p: "/etc/hosts", want: testRepoPath + "/etc/hosts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToInternalPath(testRepoPath, tt.p); got != tt.want {
				t.Errorf("ToInternalPath(%q) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

// A home directory of "/" puts every path under $HOME, and must not produce a
// doubled separator on the way back
func TestPathConversionWithRootAsHome(t *testing.T) {
	setHomeDir(t, "/")
	repoPath := "/data/gog/testrepo"

	internal := ToInternalPath(repoPath, "/etc/foo")

	if want := repoPath + "/$HOME/etc/foo"; internal != want {
		t.Errorf("ToInternalPath() = %q, want %q", internal, want)
	}
	if got := ToExternalPath(repoPath, internal); got != "/etc/foo" {
		t.Errorf("ToExternalPath() = %q, want /etc/foo", got)
	}
}
