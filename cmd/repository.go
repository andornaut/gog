package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/andornaut/gog/git"
	"github.com/andornaut/gog/repository"
)

// RunAddRepository runs the `gog repository add` command
func RunAddRepository(repoName string, repoURL string) error {
	if err := repository.ValidateRepoName(repoName); err != nil {
		return err
	}

	repoPath := path.Join(repository.BaseDir, repoName)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return err
	}

	if repoURL == "" {
		if err := git.Init(repoPath); err != nil {
			return err
		}
	} else {
		if err := git.Clone(repoPath, repoURL); err != nil {
			return err
		}
	}
	fmt.Println(repoPath)
	return nil
}

// RunRemoveRepository runs the `gog repository remove` command
func RunRemoveRepository(repoName string) error {
	if err := repository.ValidateRepoName(repoName); err != nil {
		return err
	}

	repoPath := path.Join(repository.BaseDir, repoName)
	if err := os.RemoveAll(repoPath); err != nil {
		return err
	}
	fmt.Println(repoPath)
	return nil
}

// RunGetDefaultRepository runs the `gog repository get-default` command
func RunGetDefaultRepository(isPath bool) error {
	repoPath, err := repository.GetDefault()
	if err != nil {
		return err
	}

	if isPath {
		fmt.Println(repoPath)
	} else {
		fmt.Println(filepath.Base(repoPath))
	}
	return nil
}

// RunListRepositories runs the `gog repository list` command
func RunListRepositories(isPath bool) error {
	entries, err := ioutil.ReadDir(repository.BaseDir)
	if err != nil {
		return err
	}

	for _, fileInfo := range entries {
		if fileInfo.IsDir() {
			msg := fileInfo.Name()
			if isPath {
				msg = path.Join(repository.BaseDir, msg)
			}
			fmt.Println(msg)
		}
	}
	return nil
}
