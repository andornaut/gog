package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newBaseDir points the package at an empty data directory for the duration of
// the test
func newBaseDir(t *testing.T) string {
	t.Helper()
	original := BaseDir
	BaseDir = t.TempDir()
	t.Cleanup(func() { BaseDir = original })
	return BaseDir
}

// A repository is named rather than addressed, so a name that is a path is
// refused
func TestValidateRepoName(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{name: "dotfiles", ok: true},
		{name: "with-dash_and_1", ok: true},
		{name: "../outside"},
		{name: "sub/repo"},
		{name: "with space"},
		{name: ""},
	}
	for _, tt := range tests {
		err := validateRepoName(tt.name)
		if (err == nil) != tt.ok {
			t.Errorf("validateRepoName(%q) = %v, want valid = %v", tt.name, err, tt.ok)
		}
	}
}

func TestRootPathRefusesAPathAsAName(t *testing.T) {
	newBaseDir(t)

	_, err := RootPath("../outside")

	if err == nil || !strings.Contains(err.Error(), "invalid repository name") {
		t.Errorf("RootPath(\"../outside\") = %v, want the name to be refused", err)
	}
}

// A prefix that names more than one repository is refused rather than resolved
// to whichever comes first
func TestRootPathRefusesAnAmbiguousPrefix(t *testing.T) {
	base := newBaseDir(t)
	newRepo(t, filepath.Join(base, "myrepo-v1"))
	newRepo(t, filepath.Join(base, "myrepo-v2"))

	_, err := RootPath("myrepo")

	// The candidates are named, so that the whole name to use is in front of
	// the reader rather than left to `gog repository list`.
	if err == nil || !strings.Contains(err.Error(), "myrepo-v1, myrepo-v2") {
		t.Errorf("RootPath(\"myrepo\") = %v, want a failure naming the candidates", err)
	}
	if got, err := RootPath("myrepo-v1"); err != nil || got != filepath.Join(base, "myrepo-v1") {
		t.Errorf("RootPath(\"myrepo-v1\") = %q (%v), want the repository it names", got, err)
	}
}

// The default repository is chosen with the same validation as List, so a
// directory that is not a git repository is passed over rather than selected
func TestGetFirstSkipsWhatIsNotARepository(t *testing.T) {
	base := newBaseDir(t)

	if _, err := getFirst(); err == nil {
		t.Error("getFirst() resolved a repository in an empty data directory")
	}

	// "aaa" sorts first, but only "bbb" is a git repository
	if err := os.Mkdir(filepath.Join(base, "aaa"), 0755); err != nil {
		t.Fatal(err)
	}
	newRepo(t, filepath.Join(base, "bbb"))

	got, err := getFirst()

	if err != nil {
		t.Fatalf("getFirst() = %v", err)
	}
	if want := filepath.Join(base, "bbb"); got != want {
		t.Errorf("getFirst() = %q, want %q", got, want)
	}
}

// An existing directory holding anything is never converted into a repository,
// so that a data directory is not adopted by name alone. An empty one, such as
// a failed clone leaves behind, may be reused.
func TestAddRefusesAnExistingNonEmptyDirectory(t *testing.T) {
	base := newBaseDir(t)
	occupied := filepath.Join(base, "occupied")
	if err := os.Mkdir(occupied, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(base, "empty")
	if err := os.Mkdir(empty, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Add("occupied", "")

	if err == nil || !strings.Contains(err.Error(), "not a gog repository") {
		t.Errorf("Add(\"occupied\") = %v, want the path named as not a gog repository", err)
	}
	if got, err := Add("empty", ""); err != nil || got != empty {
		t.Errorf("Add(\"empty\") = %q (%v), want the empty directory reused", got, err)
	}
}
