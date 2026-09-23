package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"init", "-q"}, args...)...)
	cmd.Env = Env()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %v: %v: %s", args, err, out)
	}
}

// Only the root of a work tree can be linked from, so a subdirectory, a plain
// directory and a bare repository are all rejected
func TestIs(t *testing.T) {
	root := t.TempDir()
	// Is runs git with gog's environment, which leaves the configuration git
	// finds through $HOME
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	repoPath := filepath.Join(root, "repo")
	subPath := filepath.Join(repoPath, "sub")
	plainPath := filepath.Join(root, "plain")
	barePath := filepath.Join(root, "bare.git")
	for _, p := range []string{subPath, plainPath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	gitInit(t, repoPath)
	gitInit(t, "--bare", barePath)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "the root of a work tree", path: repoPath, want: true},
		{name: "a subdirectory of one", path: subPath},
		{name: "a directory that is not a repository", path: plainPath},
		{name: "a bare repository", path: barePath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := Is(tt.path); got != tt.want || err != nil {
				t.Errorf("Is(%s) = %v, %v, want %v", tt.name, got, err, tt.want)
			}
		})
	}

	// An enclosing invocation such as a git hook exports GIT_DIR, which would
	// otherwise answer for the directory that was asked about
	t.Setenv("GIT_DIR", filepath.Join(repoPath, ".git"))
	if got, _ := Is(plainPath); got {
		t.Error("Is() answered for GIT_DIR rather than the directory it was given")
	}
	if got, _ := Is(repoPath); !got {
		t.Error("Is() = false for a repository root while GIT_DIR is set")
	}
}

// The variables that bind git to a repository, an index, or a configuration
// source are removed; the ones that carry transport and identity are kept, or
// clone and push would stop working
func TestEnvScrubsInheritedGitVars(t *testing.T) {
	removed := []string{
		"GIT_DIR", "GIT_INDEX_FILE", "GIT_PREFIX",
		"GIT_CONFIG_GLOBAL", "GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
	}
	// PATH is inherited rather than set here: replacing it would change how this
	// process finds git
	kept := []string{"GIT_SSH_COMMAND", "PATH"}
	for _, name := range removed {
		t.Setenv(name, "set")
	}
	t.Setenv("GIT_SSH_COMMAND", "ssh -v")

	got := map[string]bool{}
	for _, kv := range Env() {
		name, _, _ := strings.Cut(kv, "=")
		got[name] = true
	}

	for _, name := range removed {
		if got[name] {
			t.Errorf("Env() kept %s", name)
		}
	}
	for _, name := range kept {
		if !got[name] {
			t.Errorf("Env() removed %s", name)
		}
	}
}

// A repository that git refuses over its owner is reported with git's reason,
// rather than as a directory that holds no repository
func TestIsReportsARepositoryGitRefusesToUse(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	repoPath := filepath.Join(root, "repo")
	gitInit(t, repoPath)
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1")

	got, err := Is(repoPath)

	if got || !errors.Is(err, ErrUnsafe) || !strings.Contains(err.Error(), "safe.directory") {
		t.Errorf("Is() = %v, %v, want ErrUnsafe naming safe.directory", got, err)
	}
}

// gog names files literally, so a name that would be a glob matches itself
// alone, whatever the environment says about pathspecs
func TestRunGivesPathsLiterally(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("GIT_GLOB_PATHSPECS", "1")
	repoPath := filepath.Join(root, "repo")
	gitInit(t, repoPath)
	for _, name := range []string{"notes1.txt", "notes[1].txt"} {
		if err := os.WriteFile(filepath.Join(repoPath, name), []byte("x\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := Run(repoPath, "add", "notes[1].txt"); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	out, err := Output(repoPath, "ls-files")
	if err != nil {
		t.Fatal(err)
	}
	if out != "notes[1].txt\n" {
		t.Errorf("staged %q, want only notes[1].txt", out)
	}
}

// A command named by the configuration git reads through $HOME never runs from
// one of gog's own commands: apply can link that configuration into place
func TestRunRunsNoConfiguredCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	marker := filepath.Join(root, "ran")
	config := "[core]\n\tfsmonitor = \"touch " + marker + "; false\"\n"
	if err := os.WriteFile(filepath.Join(root, ".gitconfig"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(root, "repo")
	gitInit(t, repoPath)
	if err := os.WriteFile(filepath.Join(repoPath, "f"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Run(repoPath, "add", "f"); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if _, err := Output(repoPath, "status", "--porcelain"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("the configured fsmonitor ran (%v)", err)
	}
}
