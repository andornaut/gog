package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCleanPaths(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	// A relative argument is resolved against the current directory, and the
	// current directory itself may be reached through a symbolic link
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

// An empty argument used to be passed over, which left the command reporting
// success for work it never did
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
