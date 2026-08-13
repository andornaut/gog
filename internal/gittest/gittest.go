// Package gittest runs git in tests, apart from the configuration and identity
// of whoever runs them.
package gittest

import (
	"os"
	"os/exec"
	"testing"
)

// hermetic is what git is told about its configuration and the person running
// it. Without this a developer's own settings decide whether a commit is
// signed, which hooks run, and what is ignored, none of which is what a gog
// test is about.
var hermetic = []string{
	"GIT_CONFIG_GLOBAL=/dev/null",
	"GIT_CONFIG_SYSTEM=/dev/null",
	"GIT_AUTHOR_NAME=Test User",
	"GIT_AUTHOR_EMAIL=test@example.com",
	"GIT_COMMITTER_NAME=Test User",
	"GIT_COMMITTER_EMAIL=test@example.com",
}

// Init creates a git repository at repoPath, creating the directory if it is
// not there
func Init(t *testing.T, repoPath string) {
	t.Helper()
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	Run(t, repoPath, "init", "-q")
}

// Run runs a git command in repoPath and returns its output, failing the test
// if git does
func Run(t *testing.T, repoPath string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), hermetic...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// Isolate points the git that gog itself runs at an empty configuration. gog
// removes the GIT_CONFIG_* variables from git's environment so that an
// enclosing invocation cannot redirect it, which leaves the configuration git
// finds through $HOME.
func Isolate(t *testing.T, homeDir string) {
	t.Helper()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", homeDir)
}
