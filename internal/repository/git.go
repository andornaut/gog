package repository

import (
	"os"
	"os/exec"
)

// GitClone clones a git repostory
func GitClone(repoPath string, repoURL string) error {
	return GitRun(BaseDir, "clone", repoURL, repoPath)
}

// GitInit initializes a git repository
func GitInit(repoPath string) error {
	return GitRun(BaseDir, "init", repoPath)
}

// GitRun runs a git command in a repository
func GitRun(cwd string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cwd
	return cmd.Run()
}
