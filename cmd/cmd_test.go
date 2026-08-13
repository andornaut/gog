package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// setupResolveGitPaths creates a repository holding one file and a symbolic
// link to it from outside, and makes the link's directory current, so that both
// absolute and relative arguments can be exercised
func setupResolveGitPaths(t *testing.T) (repoPath, linkPath string) {
	t.Helper()
	root := t.TempDir()
	repoPath = filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	intPath := filepath.Join(repoPath, "tracked")
	if err := os.WriteFile(intPath, []byte("contents\n"), 0644); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	linkPath = filepath.Join(home, "tracked")
	if err := os.Symlink(intPath, linkPath); err != nil {
		t.Fatal(err)
	}
	// A link whose name looks like a flag, which is a pathspec only after a
	// separator. Its name differs from the file it links to, so a resolved
	// argument is distinguishable from one that was passed through.
	dashPath := filepath.Join(repoPath, "dashed-target")
	if err := os.WriteFile(dashPath, []byte("contents\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dashPath, filepath.Join(home, "-dashed")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(home)
	return repoPath, linkPath
}

func TestResolveGitPaths(t *testing.T) {
	repoPath, linkPath := setupResolveGitPaths(t)

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "pathspec after a separator is resolved",
			args: []string{"log", "--oneline", "--", linkPath},
			want: []string{"log", "--oneline", "--", "tracked"},
		},
		{
			name: "relative pathspec after a separator is resolved",
			args: []string{"log", "--", "./tracked"},
			want: []string{"log", "--", "tracked"},
		},
		{
			name: "operand of a pathspec subcommand is resolved",
			args: []string{"add", linkPath},
			want: []string{"add", "tracked"},
		},
		{
			name: "flag of a pathspec subcommand is passed through",
			args: []string{"add", "--force", linkPath},
			want: []string{"add", "--force", "tracked"},
		},
		{
			name: "operand of any other subcommand is passed through",
			args: []string{"branch", "tracked"},
			want: []string{"branch", "tracked"},
		},
		{
			name: "value of a flag is passed through",
			args: []string{"commit", "-m", "tracked"},
			want: []string{"commit", "-m", "tracked"},
		},
		{
			name: "subcommand is passed through",
			args: []string{"tracked"},
			want: []string{"tracked"},
		},
		{
			name: "nothing before a separator is resolved without a pathspec subcommand",
			args: []string{"checkout", "tracked", "--", "tracked"},
			want: []string{"checkout", "tracked", "--", "tracked"},
		},
		{
			name: "a pathspec that looks like a flag is resolved after a separator",
			args: []string{"log", "--", "-dashed"},
			want: []string{"log", "--", "dashed-target"},
		},
		{
			name: "a name that looks like a flag is passed through before a separator",
			args: []string{"add", "-dashed"},
			want: []string{"add", "-dashed"},
		},
		{
			name: "a path outside the repository is passed through",
			args: []string{"add", ".."},
			want: []string{"add", ".."},
		},
		{
			name: "a nonexistent path is passed through",
			args: []string{"add", "missing"},
			want: []string{"add", "missing"},
		},
		{
			name: "a global flag suppresses subcommand detection",
			args: []string{"-C", "elsewhere", "add", "tracked"},
			want: []string{"-C", "elsewhere", "add", "tracked"},
		},
		{
			name: "no arguments",
			args: []string{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveGitPaths(repoPath, tt.args)
			if !slices.Equal(got, tt.want) {
				t.Errorf("resolveGitPaths(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

// A repository can be reached through a symlinked parent directory: `/var` and
// `/tmp` are symbolic links on macOS, so a temporary directory is one there.
// The repository path and the resolved argument have to be compared in the same
// terms, or no pathspec is ever converted.
func TestResolveGitPathsThroughASymlinkedRepositoryPath(t *testing.T) {
	root := t.TempDir()
	realRoot := filepath.Join(root, "real")
	repoPath := filepath.Join(realRoot, "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	intPath := filepath.Join(repoPath, "tracked")
	if err := os.WriteFile(intPath, []byte("contents\n"), 0644); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(root, "linked")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}

	args := []string{"add", intPath}
	got := resolveGitPaths(filepath.Join(linkedRoot, "repo"), args)
	want := []string{"add", "tracked"}
	if !slices.Equal(got, want) {
		t.Errorf("resolveGitPaths(%q) = %q, want %q", args, got, want)
	}
}

func TestResolveGitPathsDoesNotMutateItsArgument(t *testing.T) {
	repoPath, linkPath := setupResolveGitPaths(t)

	args := []string{"add", linkPath}
	resolveGitPaths(repoPath, args)
	if args[1] != linkPath {
		t.Errorf("args[1] = %q, want %q", args[1], linkPath)
	}
}

// `gog git` hands every argument to git, so the flag that selects the
// repository is read here rather than by cobra, and only where git could not
// mean it
func TestTakeRepositoryFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantArgs []string
	}{
		{name: "a separate value", args: []string{"-r", "work", "status"}, wantName: "work", wantArgs: []string{"status"}},
		{name: "the long form", args: []string{"--repository", "work", "status"}, wantName: "work", wantArgs: []string{"status"}},
		{name: "an attached value", args: []string{"-rwork", "status"}, wantName: "work", wantArgs: []string{"status"}},
		{name: "the long form with an equals sign", args: []string{"--repository=work", "status"}, wantName: "work", wantArgs: []string{"status"}},
		{name: "git's own -r is left alone", args: []string{"branch", "-r"}, wantArgs: []string{"branch", "-r"}},
		{name: "and so is one that follows a subcommand", args: []string{"ls-tree", "-r", "HEAD"}, wantArgs: []string{"ls-tree", "-r", "HEAD"}},
		{name: "no arguments at all", args: []string{}, wantArgs: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repositoryFlag = ""
			got, err := takeRepositoryFlag(tt.args)
			if err != nil {
				t.Fatalf("takeRepositoryFlag(%q) = %v", tt.args, err)
			}
			if repositoryFlag != tt.wantName {
				t.Errorf("selected repository %q, want %q", repositoryFlag, tt.wantName)
			}
			if !slices.Equal(got, tt.wantArgs) {
				t.Errorf("takeRepositoryFlag(%q) = %q, want %q", tt.args, got, tt.wantArgs)
			}
		})
	}
}

func TestTakeRepositoryFlagWithoutAName(t *testing.T) {
	repositoryFlag = ""
	if _, err := takeRepositoryFlag([]string{"-r"}); err == nil {
		t.Error("takeRepositoryFlag() accepted a flag with nothing after it")
	}
}

// Only `gog git` reports a status of its own; every other failure is gog's
func TestExitCode(t *testing.T) {
	if got := ExitCode(&exitCodeError{code: 128}); got != 128 {
		t.Errorf("ExitCode() = %d, want 128", got)
	}
	if got := ExitCode(fmt.Errorf("wrapped: %w", &exitCodeError{code: 3})); got != 3 {
		t.Errorf("ExitCode() of a wrapped status = %d, want 3", got)
	}
	if got := ExitCode(errors.New("gog's own failure")); got != 1 {
		t.Errorf("ExitCode() = %d, want 1", got)
	}
}
