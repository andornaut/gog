package repository

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ValidateRepoName returns an error if the repo name is invalid
func ValidateRepoName(name string) error {
	validRepoName := regexp.MustCompile(`^[\w-_]+$`)
	if !validRepoName.MatchString(name) {
		return fmt.Errorf("Invalid repository name: %s", name)
	}
	return nil
}

func validateRepoPath(p string) error {
	fileInfo, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("Invalid repository path: %s", p)
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("Repository path must be a directory: %s", p)
	}
	return nil
}

func validateTargetPath(p string) error {
	if shouldSkip(p, "") {
		return fmt.Errorf("Invalid target path: %s", p)
	}
	return nil
}

func shouldSkip(extPath, _ string) bool {
	return strings.HasPrefix(extPath, BaseDir) || strings.HasSuffix(extPath, ".gog")
}
