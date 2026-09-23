package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/fscopy"
	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/paths"
)

var (
	// validRepoName is the pattern a repository name must match. \w already
	// includes underscores.
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
		return fmt.Errorf("repository %q not found", filepath.Base(p))
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("repository path %q must be a directory", p)
	}
	isRepo, err := git.Is(p)
	if err != nil {
		return fmt.Errorf("repository %q: %w", filepath.Base(p), err)
	}
	if !isRepo {
		return fmt.Errorf("repository %q must be initialized as a git repository (run \"git init\" in it)", p)
	}
	return nil
}

func validateTargetPath(p string) error {
	if paths.Within(BaseDir, p) {
		return ownPathError(p)
	}
	// A path can reach the data directory through a symbolically linked parent,
	// such as ~/dots pointing at a repository. Only the parent is resolved: a
	// link that gog made resolves into the data directory, and is a path that
	// gog manages rather than one inside it.
	if resolved := paths.ResolveParent(p); WithinBaseDir(resolved) {
		return ownPathError(resolved)
	}
	// .gog is the suffix of a backup that older versions left behind, which
	// duplicates a file the repository already holds
	if strings.HasSuffix(p, ".gog") {
		return fmt.Errorf("invalid target path %q (.gog backup files cannot be managed)", p)
	}
	return nil
}

// ownPathError refuses a path inside gog's data directory: a repository's copy
// is named by the path it is linked from. That path is given too when the link
// is there to prove which one it is.
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

// skipFor returns what a copy of the directory typedRoot, which resolves to
// resolvedRoot, passes over.
//
// The copy hands it paths that are already resolved, so the data directory must
// be resolved too: a home directory reached through a symbolic link spells
// BaseDir one way and the walk another, and gog would then copy its own data
// directory into the repository it is adding to.
//
// A nested repository's .git is passed over with a warning. Its files are the
// other repository's own, and linking them breaks it: git refuses a HEAD that
// is a symbolic link.
func skipFor(resolvedRoot, typedRoot string) fscopy.SkipFunc {
	return func(extPath, _ string) bool {
		if filepath.Base(extPath) == ".git" {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s (a nested git repository's own files cannot be managed)\n",
				paths.Display(asTyped(extPath, resolvedRoot, typedRoot)))
			return true
		}
		return WithinBaseDir(extPath) || strings.HasSuffix(extPath, ".gog")
	}
}
