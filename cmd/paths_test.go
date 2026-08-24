package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/andornaut/gog/internal/gittest"
	"github.com/andornaut/gog/internal/repository"
	"github.com/andornaut/gog/internal/testout"
)

func TestCleanPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	// Read back rather than reused: a temporary directory may be reached
	// through a symbolic link, and cleanPaths joins what the process reports
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "an absolute path is kept", args: []string{"/etc/hosts"}, want: []string{"/etc/hosts"}},
		{name: "a relative path is resolved", args: []string{"conf"}, want: []string{filepath.Join(cwd, "conf")}},
		{name: "a dot path is resolved", args: []string{"./sub/../conf"}, want: []string{filepath.Join(cwd, "conf")}},
		{name: "a trailing slash is dropped", args: []string{"/etc/"}, want: []string{"/etc"}},
		{name: "several paths keep their order", args: []string{"/b", "/a"}, want: []string{"/b", "/a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanPaths(tt.args)
			if err != nil {
				t.Fatalf("cleanPaths(%q) = %v", tt.args, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("cleanPaths(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

// An empty argument names nothing, and passing it over would report success
// for work that was never done
func TestCleanPathsRefusesAnEmptyPath(t *testing.T) {
	for _, arg := range []string{"", "   ", "\t"} {
		got, err := cleanPaths([]string{"/etc/hosts", arg})
		if err == nil {
			t.Errorf("cleanPaths(%q) = %q, want a failure", arg, got)
			continue
		}
		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("cleanPaths(%q) = %v, want a failure naming the empty path", arg, err)
		}
	}
}

// newSandbox creates a repository holding one linked file, and points the
// repository package at it and at a home directory for the duration of the test
func newSandbox(t *testing.T) (repoPath, extPath string) {
	t.Helper()
	root := t.TempDir()
	baseDir := filepath.Join(root, "data")
	repoPath = filepath.Join(baseDir, "dots")
	homeDir := filepath.Join(root, "home")
	intPath := filepath.Join(repoPath, repository.ContentDirName, "$HOME", ".bashrc")
	for _, p := range []string{filepath.Dir(intPath), homeDir} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(intPath, []byte("bashrc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	extPath = filepath.Join(homeDir, ".bashrc")
	if err := os.Symlink(intPath, extPath); err != nil {
		t.Fatal(err)
	}
	gittest.Init(t, repoPath)
	gittest.Isolate(t, homeDir)
	// The repository and link packages read these. Cleared here, so a host that
	// has gog configured for its own use does not decide what the test sees.
	t.Setenv("GOG_DEFAULT_REPOSITORY_NAME", "")
	t.Setenv("GOG_HOME", "")
	t.Setenv("GOG_IGNORE_FILES_REGEX", "")

	originalBase := repository.BaseDir
	repository.BaseDir = baseDir
	originalHome := repository.SetHomeDirForTest(homeDir)
	t.Cleanup(func() {
		repository.BaseDir = originalBase
		repository.SetHomeDirForTest(originalHome)
	})
	return repoPath, extPath
}

// `gog list` prints the paths a repository holds on standard output, which is
// what a caller reads, and -s prefixes each with what applying would do to it
func TestListPrintsWhatApplyingWouldDo(t *testing.T) {
	_, extPath := newSandbox(t)
	tests := []struct {
		name   string
		status bool
		want   string
	}{
		{name: "the paths alone", want: extPath + "\n"},
		{name: "with what applying would do", status: true, want: "linked   " + extPath + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set through the flag rather than the variable behind it, so that
			// the flag the command registered decides what its run reads
			if err := list.Flags().Set("status", strconv.FormatBool(tt.status)); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := list.Flags().Set("status", "false"); err != nil {
					t.Fatal(err)
				}
			})
			var out bytes.Buffer
			list.SetOut(&out)
			t.Cleanup(func() { list.SetOut(nil) })

			if err := list.RunE(list, nil); err != nil {
				t.Fatalf("list = %v", err)
			}

			if got := out.String(); got != tt.want {
				t.Errorf("list printed %q, want %q", got, tt.want)
			}
		})
	}
}

// The batch is checked before anything is restored, so that an unusable path
// does not leave the paths before it half-removed
func TestRmChecksTheBatchBeforeRestoringAnything(t *testing.T) {
	repoPath, extPath := newSandbox(t)
	inRepository := filepath.Join(repoPath, repository.ContentDirName, "$HOME", ".bashrc")

	err := rm.RunE(rm, []string{extPath, inRepository})

	if err == nil || !strings.Contains(err.Error(), "repository dots holds it") {
		t.Fatalf("rm = %v, want the batch refused", err)
	}
	if target, readErr := os.Readlink(extPath); readErr != nil || target != inRepository {
		t.Errorf("%s -> %q (%v), want the link left in place", extPath, target, readErr)
	}
}

// A run that linked nothing and one that linked everything are otherwise the
// same, both silent and exit 0
func TestNoteEmpty(t *testing.T) {
	want := "Note: dots holds no root/, so there is nothing to link\n"
	tests := []struct {
		name string
		// prepare puts something at the content path, or nothing when it is nil
		prepare func(t *testing.T, contentPath string)
		want    string
	}{
		{name: "no content directory", want: want},
		{
			// Walking it would not be walking a tree
			name: "a regular file of that name",
			prepare: func(t *testing.T, contentPath string) {
				t.Helper()
				if err := os.WriteFile(contentPath, []byte("x"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			want: want,
		},
		{
			name: "a content directory",
			prepare: func(t *testing.T, contentPath string) {
				t.Helper()
				if err := os.Mkdir(contentPath, 0755); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := filepath.Join(t.TempDir(), "dots")
			if err := os.Mkdir(repoPath, 0755); err != nil {
				t.Fatal(err)
			}
			if tt.prepare != nil {
				tt.prepare(t, repository.ContentPath(repoPath))
			}

			if got := testout.Capture(t, func() { noteEmpty(repoPath) }); got != tt.want {
				t.Errorf("noteEmpty() printed %q, want %q", got, tt.want)
			}
		})
	}
}
