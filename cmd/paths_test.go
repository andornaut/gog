package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
