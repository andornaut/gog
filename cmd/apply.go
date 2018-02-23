package cmd

import (
	"github.com/andornaut/gog/link"
	"github.com/andornaut/gog/repository"
)

// RunApply runs the `gog apply` command
func RunApply(repoName string) error {
	repoPath, err := repository.RootPath(repoName)
	if err != nil {
		return err
	}

	return link.Dir(repoPath, repoPath)
}
