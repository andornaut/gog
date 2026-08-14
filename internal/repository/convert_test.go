package repository

import (
	"os"
	"path/filepath"
	"strings"
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

// newRepoDir builds a repository directory on disk, since the layout is read
// from it. A legacy one keeps its content at its top level; the current one
// keeps it under ContentDirName.
func newRepoDir(t *testing.T, legacy bool) string {
	t.Helper()
	repoPath := filepath.Join(t.TempDir(), "testrepo")
	held := filepath.Join(repoPath, ContentDirName, "$HOME")
	if legacy {
		held = filepath.Join(repoPath, "$HOME")
	}
	if err := os.MkdirAll(held, 0755); err != nil {
		t.Fatal(err)
	}
	return repoPath
}

// layouts runs a test against a repository of each layout, the conversion rules
// being the same in both and only the directory they are rooted at differing.
func layouts(t *testing.T, run func(t *testing.T, repoPath string)) {
	t.Helper()
	for _, tc := range []struct {
		name   string
		legacy bool
	}{
		{name: "content directory", legacy: false},
		{name: "legacy layout", legacy: true},
	} {
		t.Run(tc.name, func(t *testing.T) { run(t, newRepoDir(t, tc.legacy)) })
	}
}

// Only $HOME is expanded. Expanding whatever a component names would let the
// environment decide where a repository writes.
func TestToExternalPath(t *testing.T) {
	setHomeDir(t, testHomeDir)

	layouts(t, func(t *testing.T, repoPath string) {
		content := ContentPath(repoPath)
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
	})
}

// A path under the home directory is stored by the portable $HOME component,
// and the home directory is matched on a path boundary so that a sibling of it
// is not read as being within it
func TestToInternalPath(t *testing.T) {
	setHomeDir(t, testHomeDir)

	layouts(t, func(t *testing.T, repoPath string) {
		content := ContentPath(repoPath)
		tests := []struct {
			name string
			p    string
			want string
		}{
			{name: "within home", p: "/home/testuser/.bashrc", want: content + "/$HOME/.bashrc"},
			{name: "a sibling of home", p: "/home/testuserother/.bashrc", want: content + "/home/testuserother/.bashrc"},
			{name: "outside home", p: "/etc/hosts", want: content + "/etc/hosts"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := ToInternalPath(repoPath, tt.p); got != tt.want {
					t.Errorf("ToInternalPath(%q) = %q, want %q", tt.p, got, tt.want)
				}
			})
		}
	})
}

// A home directory of "/" puts every path under $HOME, and must not produce a
// doubled separator on the way back
func TestPathConversionWithRootAsHome(t *testing.T) {
	setHomeDir(t, "/")
	repoPath := newRepoDir(t, false)

	internal := ToInternalPath(repoPath, "/etc/foo")

	if want := filepath.Join(repoPath, ContentDirName, "$HOME", "etc", "foo"); internal != want {
		t.Errorf("ToInternalPath() = %q, want %q", internal, want)
	}
	if got := ToExternalPath(repoPath, internal); got != "/etc/foo" {
		t.Errorf("ToExternalPath() = %q, want /etc/foo", got)
	}
}

// Which directory a repository's tree is linked from, decided by what the
// repository holds: the content directory where there is one, its own top level
// where it holds content there, and the content directory for a repository that
// holds neither, so that a new one takes the current layout.
func TestContentPath(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		legacy  bool
	}{
		{name: "a content directory", entries: []string{ContentDirName + "/$HOME/.bashrc"}},
		{name: "content at the top level", entries: []string{"$HOME/.bashrc"}, legacy: true},
		{name: "content outside home at the top level", entries: []string{"etc/hosts"}, legacy: true},
		{name: "an empty repository", entries: []string{".git/HEAD"}},
		{name: "nothing but the repository's own files",
			entries: []string{".git/HEAD", ".gitignore", "LICENSE", "README.md", ".github/workflows/test.yml"}},
		{name: "both, the content directory winning",
			entries: []string{ContentDirName + "/$HOME/.bashrc", "$HOME/.bashrc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := filepath.Join(t.TempDir(), "testrepo")
			for _, rel := range tt.entries {
				p := filepath.Join(repoPath, rel)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}

			want := filepath.Join(repoPath, ContentDirName)
			if tt.legacy {
				want = repoPath
			}
			if got := ContentPath(repoPath); got != want {
				t.Errorf("ContentPath() = %q, want %q", got, want)
			}
		})
	}
}

// A repository that cannot be read is answered for as the layout every existing
// one has: read as new, its content would be looked for in a directory it does
// not have, and nothing would be linked at all.
func TestContentPathOfAnUnreadableRepository(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "not-there")

	if got := ContentPath(repoPath); got != repoPath {
		t.Errorf("ContentPath() = %q, want %q", got, repoPath)
	}
}

// What a repository holds beside the directory whose tree is linked, which is a
// move that was not finished: nothing links it, and nothing else says so.
func TestStranded(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    []string
	}{
		{name: "a finished move", entries: []string{ContentDirName + "/$HOME/.bashrc"}},
		{name: "a repository that was never moved", entries: []string{"$HOME/.bashrc"}},
		{name: "a move that left a tree behind",
			entries: []string{ContentDirName + "/$HOME/.bashrc", "etc/hosts"}, want: []string{"etc"}},
		{name: "the repository's own files are not stranded",
			entries: []string{ContentDirName + "/$HOME/.bashrc", ".github/workflows/test.yml", "README.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := filepath.Join(t.TempDir(), "testrepo")
			for _, rel := range tt.entries {
				p := filepath.Join(repoPath, rel)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := Stranded(repoPath)

			if len(got) != len(tt.want) {
				t.Fatalf("Stranded() = %v, want %v", got, tt.want)
			}
			for i, name := range tt.want {
				if got[i] != name {
					t.Errorf("Stranded()[%d] = %q, want %q", i, got[i], name)
				}
			}
		})
	}
}

// A repository written before the content directory existed stores /root at its
// top level, under the name that decides the layout. Adding it would leave the
// next command reading the repository as one that had been moved, with one file
// under that name and every other path it holds converting to nothing.
func TestRefuseContentDirCollision(t *testing.T) {
	setHomeDir(t, testHomeDir)

	tests := []struct {
		name    string
		legacy  bool
		target  string
		refused bool
	}{
		{name: "/root into a repository that was never moved", legacy: true,
			target: "/" + ContentDirName + "/.profile", refused: true},
		{name: "/root itself into one", legacy: true, target: "/" + ContentDirName, refused: true},
		{name: "an ordinary path into one", legacy: true, target: "/etc/hosts"},
		{name: "/root into a repository that keeps its content in one place",
			target: "/" + ContentDirName + "/.profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := newRepoDir(t, tt.legacy)

			err := refuseContentDirCollision(repoPath, tt.target, ToInternalPath(repoPath, tt.target))

			if tt.refused && (err == nil || !strings.Contains(err.Error(), ContentDirName)) {
				t.Errorf("refuseContentDirCollision() = %v, want a refusal naming the content directory", err)
			}
			if !tt.refused && err != nil {
				t.Errorf("refuseContentDirCollision() = %v, want nil", err)
			}
		})
	}
}
