package repositorycmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andornaut/gog/internal/repository"
)

// Cobra's own message ("accepts between 1 and 2 arg(s), received 0") names
// neither the command nor what it wanted
func TestRequireArgs(t *testing.T) {
	want := "a repository name and an optional URL"
	check := requireArgs(1, 2, want)

	for _, args := range [][]string{{}, {"name", "url", "extra"}} {
		err := check(add, args)
		if err == nil || !strings.HasSuffix(err.Error(), "requires "+want) {
			t.Errorf("requireArgs(%q) = %v, want the command and what it wanted named", args, err)
		}
	}
	for _, args := range [][]string{{"name"}, {"name", "url"}} {
		if err := check(add, args); err != nil {
			t.Errorf("requireArgs(%q) = %v, want success", args, err)
		}
	}
}

// Naming a subcommand that does not exist is a wrong invocation, and so is
// naming none. Both are reported by the argument validator, which runs before
// usage is silenced, so the reader is shown the commands they could have named.
func TestNeedsCommand(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		// The command path is "repository" here and "gog repository" in the
		// binary, where the root has adopted it.
		{[]string{"bogus"}, `unknown command "bogus" for "repository"`},
		{nil, "repository requires a command"},
	} {
		if err := Cmd.Args(Cmd, tt.args); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Args(%q) = %v, want %q", tt.args, err, tt.want)
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
	intPath := filepath.Join(repoPath, "$HOME", ".bashrc")
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intPath, []byte("bashrc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	extPath = filepath.Join(homeDir, ".bashrc")
	if err := os.Symlink(intPath, extPath); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"add", "-A"},
		{"commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	originalBase := repository.BaseDir
	repository.BaseDir = baseDir
	originalHome := repository.SetHomeDirForTest(homeDir)
	t.Cleanup(func() {
		repository.BaseDir = originalBase
		repository.SetHomeDirForTest(originalHome)
	})
	return repoPath, extPath
}

// Deleting a repository cannot be undone by cloning again, so a repository
// holding work that no remote has is refused before anything is restored or
// deleted. A repository with no remote holds its whole history.
func TestRemoveRefusesBeforeItRestoresAnything(t *testing.T) {
	repoPath, extPath := newSandbox(t)

	err := remove.RunE(remove, []string{"dots"})

	if err == nil || !strings.Contains(err.Error(), "refusing to remove dots: it holds 1 commit that no remote has") {
		t.Fatalf("remove = %v, want a refusal counting the unsaved work", err)
	}
	if target, readErr := os.Readlink(extPath); readErr != nil || target != filepath.Join(repoPath, "$HOME", ".bashrc") {
		t.Errorf("%s -> %q (%v), want the link left in place", extPath, target, readErr)
	}
	if _, statErr := os.Stat(repoPath); statErr != nil {
		t.Errorf("the repository was deleted although the removal was refused: %v", statErr)
	}
}

// --force deletes it, and what the repository held is restored as ordinary
// files rather than left as links to nothing
func TestRemoveRestoresWhatItHeld(t *testing.T) {
	repoPath, extPath := newSandbox(t)
	isForced = true
	t.Cleanup(func() { isForced = false })

	if err := remove.RunE(remove, []string{"dots"}); err != nil {
		t.Fatalf("remove = %v", err)
	}

	if _, statErr := os.Stat(repoPath); !os.IsNotExist(statErr) {
		t.Errorf("the repository is still there (%v)", statErr)
	}
	if _, readErr := os.Readlink(extPath); readErr == nil {
		t.Errorf("%s is still a symbolic link", extPath)
	}
	if contents, readErr := os.ReadFile(extPath); readErr != nil || string(contents) != "bashrc\n" {
		t.Errorf("%s holds %q (%v), want the contents restored", extPath, contents, readErr)
	}
}
