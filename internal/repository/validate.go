package repository

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/git"
)

// validateRepoName returns an error if the repo name is invalid
func validateRepoName(name string) error {
	validRepoName := regexp.MustCompile(`^[\w-_]+$`)
	if !validRepoName.MatchString(name) {
		return fmt.Errorf("invalid repository name: %s", name)
	}
	return nil
}

func validateRepoPath(p string) error {
	fileInfo, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("invalid repository path: %s", p)
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("repository path must be a directory: %s", p)
	}
	if !git.Is(p) {
		return fmt.Errorf("repository must be initialized as a git repository")
	}
	return nil
}

func validateTargetPath(p string) error {
	if shouldSkip(p, "") {
		return fmt.Errorf("invalid target path: %s", p)
	}
	return nil
}

func shouldSkip(extPath, _ string) bool {
	return strings.HasPrefix(extPath, BaseDir) || strings.HasSuffix(extPath, ".gog")
}
