package git

import (
	"os"
	"os/exec"
)

// Clone clones a git repository
func Clone(baseDir, repoPath string, repoURL string) error {
	return Run(baseDir, "clone", repoURL, repoPath)
}

// Init initializes a git repository
func Init(baseDir, repoPath string) error {
	return Run(baseDir, "init", repoPath)
}

// Is returns true if the given directory is a git repository
func Is(baseDir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = baseDir
	err := cmd.Run()
	return err == nil
}

// Run runs a git command in a repository
func Run(baseDir string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = baseDir
	return cmd.Run()
}
