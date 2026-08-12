package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/paths"
)

var (
	// validRepoName is the regex pattern for valid repository names.
	// \w already includes underscores.
	validRepoName = regexp.MustCompile(`^[\w-]+$`)
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
		return fmt.Errorf("repository not found: %s", filepath.Base(p))
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
	if paths.Within(BaseDir, p) {
		return fmt.Errorf("invalid target path %q (gog's own directory cannot be managed)", p)
	}
	if strings.HasSuffix(p, ".gog") {
		return fmt.Errorf("invalid target path %q (.gog backup files cannot be managed)", p)
	}
	return nil
}

func shouldSkip(extPath, _ string) bool {
	return paths.Within(BaseDir, extPath) || strings.HasSuffix(extPath, ".gog")
}
