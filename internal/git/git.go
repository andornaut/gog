package git

import (
	"os"
	"os/exec"
	"strings"
)

// Clone clones a git repository
func Clone(baseDir, repoPath string, repoURL string) error {
	return Run(baseDir, "clone", repoURL, repoPath)
}

// Init initializes a git repository
func Init(baseDir, repoPath string) error {
	return Run(baseDir, "init", repoPath)
}

// Is returns true if the given directory is the root of a git repository's
// work tree. The combined invocation prints "false" for a non-bare
// repository followed by the relative path to the top level, which is empty
// at the root itself. Bare repositories have no work tree to link from, so
// they are rejected.
func Is(baseDir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-bare-repository", "--show-cdup")
	cmd.Dir = baseDir
	cmd.Env = commandEnv()
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "false"
}

// Run runs a git command in a repository
func Run(baseDir string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = baseDir
	cmd.Env = commandEnv()
	return cmd.Run()
}

// commandEnv returns the process environment without GIT_DIR and
// GIT_WORK_TREE, which would redirect git away from the repository at
// cmd.Dir (e.g. when gog is invoked from a git hook)
func commandEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_DIR=") || strings.HasPrefix(kv, "GIT_WORK_TREE=") {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}
