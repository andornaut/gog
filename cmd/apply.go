package cmd

import (
	"fmt"

	"github.com/andornaut/gog/link"
	"github.com/andornaut/gog/repository"
)

// RunApply runs the `gog apply` command
func RunApply(repoName string) error {
	repoPath, err := repository.RootPath(repoName)
	if err != nil {
		return err
	}
	fmt.Printf("repository: %s\n---\n", repoPath)

	return link.Dir(repoPath, repoPath)
}
