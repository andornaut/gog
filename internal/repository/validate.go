package repository

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/git"
)

var (
	// validRepoName is the regex pattern for valid repository names
	validRepoName = regexp.MustCompile(`^[\w-_]+$`)
)

// validateRepoName returns an error if the repo name is invalid
func validateRepoName(name string) error {
	if !validRepoName.MatchString(name) {
		return fmt.Errorf("invalid repository name %q (must contain only letters, numbers, dashes, and underscores)", name)
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
		return fmt.Errorf("repository must be initialized as a git repository (run 'git init' in %s)", p)
	}
	return nil
}

func validateTargetPath(p string) error {
	if strings.HasPrefix(p, BaseDir) {
		return fmt.Errorf("invalid target path %q (cannot add gog's own directory)", p)
	}
	if strings.HasSuffix(p, ".gog") {
		return fmt.Errorf("invalid target path %q (cannot add .gog backup files)", p)
	}
	return nil
}

func shouldSkip(extPath, _ string) bool {
	return strings.HasPrefix(extPath, BaseDir) || strings.HasSuffix(extPath, ".gog")
}
