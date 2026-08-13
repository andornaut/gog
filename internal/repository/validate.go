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
		return ownPathError(p)
	}
	// gog no longer creates .gog backups, but earlier versions left them in home
	// directories, and a displaced copy of a file the repository already holds
	// is not worth tracking
	if strings.HasSuffix(p, ".gog") {
		return fmt.Errorf("invalid target path %q (.gog backup files cannot be managed)", p)
	}
	return nil
}

// ownPathError refuses a path inside gog's data directory, which a command
// never means: a repository's copy is reached by the path it is linked from.
// The repository is named, and so is that path when the link is there to prove
// which one it is.
func ownPathError(p string) error {
	name := repoNameOf(p)
	if name == "" || name == "." {
		return fmt.Errorf("invalid target path %q (gog's own data directory cannot be managed)", p)
	}
	repoPath := filepath.Join(BaseDir, name)
	if p == repoPath {
		return fmt.Errorf("invalid target path %q (that is repository %s; name the paths it holds instead)", p, name)
	}
	if extPath := ToExternalPath(repoPath, p); linksTo(extPath, p) {
		return fmt.Errorf("invalid target path %q (repository %s holds it; name %s instead)", p, name, extPath)
	}
	return fmt.Errorf("invalid target path %q (repository %s holds it)", p, name)
}

// linksTo reports whether p is a symbolic link to target
func linksTo(p, target string) bool {
	resolved, err := os.Readlink(p)
	return err == nil && resolved == target
}

func shouldSkip(extPath, _ string) bool {
	return paths.Within(BaseDir, extPath) || strings.HasSuffix(extPath, ".gog")
}
