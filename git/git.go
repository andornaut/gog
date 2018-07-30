package git

import (
	"os"
	"os/exec"

	"github.com/andornaut/gog/repository"
)

// Clone clones a git repostory
func Clone(repoPath string, repoURL string) error {
	return RunCommand(repository.BaseDir, "clone", repoURL, repoPath)
}

// Init initializes a git repository
func Init(repoPath string) error {
	return RunCommand(repository.BaseDir, "init", repoPath)
}

// RunCommand runs a git command in a repository
func RunCommand(cwd string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cwd
	return cmd.Run()
}
