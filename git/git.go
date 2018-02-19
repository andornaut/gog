package git

import (
	"os"
	"os/exec"

	"github.com/andornaut/gog/repository"
)

// Clone clones a git repostory
func Clone(repoPath string, repoURL string) error {
	return run(repository.BaseDir, "clone", repoURL, repoPath)
}

// Init initializes a git repository
func Init(repoPath string) error {
	return run(repository.BaseDir, "init", repoPath)
}

// Run runs a git command in a repository
func Run(repoPath string, arguments []string) error {
	return run(repoPath, arguments...)
}

func run(cwd string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cwd
	return cmd.Run()
}
