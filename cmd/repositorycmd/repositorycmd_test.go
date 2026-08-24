package repositorycmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/andornaut/gog/internal/gittest"
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

// cobra.NoArgs reports an operand as an unknown command, which misdescribes
// `gog repository list extra`: list takes no operands at all
func TestNoArgs(t *testing.T) {
	err := noArgs(list, []string{"extra"})

	if err == nil || err.Error() != `repository list takes no arguments, but got "extra"` {
		t.Errorf("noArgs() = %v, want the command and the operand named", err)
	}
	if err := noArgs(list, nil); err != nil {
		t.Errorf("noArgs() = %v, want success", err)
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
	intPath := filepath.Join(repoPath, repository.ContentDirName, "$HOME", ".bashrc")
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
	gittest.Init(t, repoPath)
	gittest.Isolate(t, homeDir)
	gittest.Run(t, repoPath, "add", "-A")
	gittest.Run(t, repoPath, "commit", "-q", "-m", "init")

	// The repository package reads this. Cleared here, so a host that has gog
	// configured for its own use does not decide which repository is default.
	t.Setenv("GOG_DEFAULT_REPOSITORY_NAME", "")

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
func TestRmRefusesBeforeItRestoresAnything(t *testing.T) {
	repoPath, extPath := newSandbox(t)

	err := rm.RunE(rm, []string{"dots"})

	if err == nil || !strings.Contains(err.Error(), "refusing to remove dots: it holds 1 commit that no remote has") {
		t.Fatalf("rm = %v, want a refusal counting the unsaved work", err)
	}
	if target, readErr := os.Readlink(extPath); readErr != nil || target != filepath.Join(repoPath, repository.ContentDirName, "$HOME", ".bashrc") {
		t.Errorf("%s -> %q (%v), want the link left in place", extPath, target, readErr)
	}
	if _, statErr := os.Stat(repoPath); statErr != nil {
		t.Errorf("the repository was deleted although the removal was refused: %v", statErr)
	}
}

// setFlag sets one of a command's flags for the duration of the test, so that
// the flag a command registered decides what its run reads
func setFlag(t *testing.T, c *cobra.Command, name string, value bool) {
	t.Helper()
	set := func(v bool) {
		if err := c.Flags().Set(name, strconv.FormatBool(v)); err != nil {
			t.Fatal(err)
		}
	}
	set(value)
	t.Cleanup(func() { set(false) })
}

// Deleting cannot be undone, so the name is given in full here rather than
// resolved from a prefix as the commands that only read a repository do
func TestRmRefusesAPrefix(t *testing.T) {
	repoPath, _ := newSandbox(t)
	setFlag(t, rm, "force", true)

	err := rm.RunE(rm, []string{"dot"})

	if err == nil || !strings.Contains(err.Error(), `repository "dot" not found`) {
		t.Fatalf("rm = %v, want the prefix to be refused", err)
	}
	if _, statErr := os.Stat(repoPath); statErr != nil {
		t.Errorf("the repository was deleted although the name was a prefix: %v", statErr)
	}
}

// `list` and `default` print what a script reads, on standard output, and
// --path chooses between the repository's name and its path
func TestListAndDefaultPrintNamesOrPaths(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		path bool
	}{
		{name: "the repositories by name", cmd: list},
		{name: "the repositories by path", cmd: list, path: true},
		{name: "the default by name", cmd: getDefault},
		{name: "the default by path", cmd: getDefault, path: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, _ := newSandbox(t)
			// Set through the flag rather than the variable behind it, so that
			// each command is held to the one it registered
			setFlag(t, tt.cmd, "path", tt.path)
			var out bytes.Buffer
			tt.cmd.SetOut(&out)
			t.Cleanup(func() { tt.cmd.SetOut(nil) })

			if err := tt.cmd.RunE(tt.cmd, nil); err != nil {
				t.Fatalf("%s = %v", tt.cmd.Name(), err)
			}

			want := "dots\n"
			if tt.path {
				want = repoPath + "\n"
			}
			if got := out.String(); got != want {
				t.Errorf("%s printed %q, want %q", tt.cmd.Name(), got, want)
			}
		})
	}
}

// --force deletes it, and what the repository held is restored as ordinary
// files rather than left as links to nothing
func TestRmRestoresWhatItHeld(t *testing.T) {
	repoPath, extPath := newSandbox(t)
	setFlag(t, rm, "force", true)

	if err := rm.RunE(rm, []string{"dots"}); err != nil {
		t.Fatalf("rm = %v", err)
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
