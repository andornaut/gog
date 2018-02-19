package cmd

import (
	"fmt"

	"github.com/andornaut/gog/git"
	"github.com/andornaut/gog/repository"
)

// RunGit runs the `gog git` command
func RunGit(repoName string, arguments []string) error {
	repoPath, err := repository.RootPath(repoName)
	if err != nil {
		return err
	}
	fmt.Printf("repository: %s\n---\n", repoPath)

	return git.Run(repoPath, arguments)
}
