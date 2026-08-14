package repository

import (
	"os"
	"path/filepath"
	"testing"
)

const testHomeDir = "/home/testuser"

// setHomeDir points the package at a home directory for the duration of the
// test
func setHomeDir(t *testing.T, home string) {
	t.Helper()
	original := SetHomeDirForTest(home)
	t.Cleanup(func() { SetHomeDirForTest(original) })
}

// newRepoDir builds a repository directory that keeps its content under
// ContentDirName, which is where every repository keeps it.
func newRepoDir(t *testing.T) string {
	t.Helper()
	repoPath := filepath.Join(t.TempDir(), "testrepo")
	if err := os.MkdirAll(filepath.Join(repoPath, ContentDirName, "$HOME"), 0755); err != nil {
		t.Fatal(err)
	}
	return repoPath
}

// Only $HOME is expanded. Expanding whatever a component names would let the
// environment decide where a repository writes.
func TestToExternalPath(t *testing.T) {
	setHomeDir(t, testHomeDir)
	content := ContentPath(newRepoDir(t))
	repoPath := filepath.Dir(content)

	tests := []struct {
		name string
		p    string
		want string
	}{
		{name: "$HOME is expanded", p: content + "/$HOME/.bashrc", want: "/home/testuser/.bashrc"},
		{name: "$PATH is not", p: content + "/$PATH/file", want: "/$PATH/file"},
		{name: "$USER is not", p: content + "/$USER/.config", want: "/$USER/.config"},
		{name: "a name that merely begins with $HOME is not", p: content + "/$HOMEWORK/file", want: "/$HOMEWORK/file"},
		{name: "anything else belongs to the filesystem root", p: content + "/etc/hosts", want: "/etc/hosts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToExternalPath(repoPath, tt.p); got != tt.want {
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
	repoPath := newRepoDir(t)
	content := ContentPath(repoPath)

	tests := []struct {
		name string
		p    string
		want string
	}{
		{name: "within home", p: "/home/testuser/.bashrc", want: content + "/$HOME/.bashrc"},
		{name: "a sibling of home", p: "/home/testuserother/.bashrc", want: content + "/home/testuserother/.bashrc"},
		{name: "outside home", p: "/etc/hosts", want: content + "/etc/hosts"},
		// /root is the superuser's home, and names the content directory only
		// because that directory is named for the filesystem root it stands for
		{name: "the superuser's home", p: "/root/.profile", want: content + "/root/.profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToInternalPath(repoPath, tt.p); got != tt.want {
				t.Errorf("ToInternalPath(%q) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

// A home directory of "/" puts every path under $HOME, and must not produce a
// doubled separator on the way back
func TestPathConversionWithRootAsHome(t *testing.T) {
	setHomeDir(t, "/")
	repoPath := newRepoDir(t)

	internal := ToInternalPath(repoPath, "/etc/foo")

	if want := filepath.Join(repoPath, ContentDirName, "$HOME", "etc", "foo"); internal != want {
		t.Errorf("ToInternalPath() = %q, want %q", internal, want)
	}
	if got := ToExternalPath(repoPath, internal); got != "/etc/foo" {
		t.Errorf("ToExternalPath() = %q, want /etc/foo", got)
	}
}
