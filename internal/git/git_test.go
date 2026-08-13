package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"init", "-q"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %v: %v: %s", args, err, out)
	}
}

// Only the root of a work tree can be linked from, so a subdirectory, a plain
// directory and a bare repository are all rejected
func TestIs(t *testing.T) {
	root := t.TempDir()
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
			if got := Is(tt.path); got != tt.want {
				t.Errorf("Is(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}

	// An enclosing invocation such as a git hook exports GIT_DIR, which would
	// otherwise answer for the directory that was asked about
	t.Setenv("GIT_DIR", filepath.Join(repoPath, ".git"))
	if Is(plainPath) {
		t.Error("Is() answered for GIT_DIR rather than the directory it was given")
	}
	if !Is(repoPath) {
		t.Error("Is() = false for a repository root while GIT_DIR is set")
	}
}

// The variables that bind git to a repository, an index, or a configuration
// source are removed; the ones that carry transport and identity are kept, or
// clone and push would stop working
func TestCommandEnvScrubsInheritedGitVars(t *testing.T) {
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
	for _, kv := range commandEnv() {
		name, _, _ := strings.Cut(kv, "=")
		got[name] = true
	}

	for _, name := range removed {
		if got[name] {
			t.Errorf("commandEnv() kept %s", name)
		}
	}
	for _, name := range kept {
		if !got[name] {
			t.Errorf("commandEnv() removed %s", name)
		}
	}
}
