package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/copy"
	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/paths"
)

// Add adds a new repository
func Add(repoName, repoURL string) (string, error) {
	if err := validateRepoName(repoName); err != nil {
		return "", err
	}

	// Reject any existing non-empty path, git repository or not, so that an
	// existing data directory is never converted into a repository. An empty
	// directory (e.g. left over from a failed clone) may be reused.
	repoPath := filepath.Join(BaseDir, repoName)
	entries, err := os.ReadDir(repoPath)
	switch {
	case err == nil && len(entries) > 0:
		// Distinguish an already-configured repository from an unrelated
		// directory so the user is not told to remove real data
		if git.Is(repoPath) {
			return "", fmt.Errorf("repository already exists: %s", repoPath)
		}
		return "", fmt.Errorf("path already exists and is not a gog repository: %s (remove it or choose another name)", repoPath)
	case err != nil && !os.IsNotExist(err):
		return "", fmt.Errorf("invalid repository path %s: %w", repoPath, err)
	}

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", err
	}

	if repoURL == "" {
		if err := git.Init(BaseDir, repoPath); err != nil {
			return "", err
		}
	} else {
		if err := git.Clone(BaseDir, repoPath, repoURL); err != nil {
			return "", err
		}
	}
	return repoPath, nil
}

// Remove removes an existing repository. The name is resolved the same way as
// --repository, so a unique prefix is accepted. It is validated first because
// RootPath treats an empty name as a request for the default repository, which
// must never be deleted by omission.
func Remove(repoName string) (string, error) {
	if err := validateRepoName(repoName); err != nil {
		return "", err
	}
	repoPath, err := RootPath(repoName)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(repoPath); err != nil {
		return "", err
	}
	return repoPath, nil
}

// AddPaths adds the given paths from the given repository. Every path is
// checked before any is copied, so that one unusable path fails the command
// outright instead of leaving the repository holding files that were never
// linked or staged.
func AddPaths(repoPath string, targetPaths []string) error {
	for _, targetPath := range targetPaths {
		if _, err := resolveAddPath(targetPath); err != nil {
			return err
		}
	}
	return syncRepository(repoPath, targetPaths, addPath)
}

// RemovePaths removes the given paths from the given repository
func RemovePaths(repoPath string, targetPaths []string) error {
	return syncRepository(repoPath, targetPaths, removePath)
}

// resolveAddPath validates a path given to `gog add` and returns the path
// whose contents will be copied into the repository
func resolveAddPath(targetPath string) (string, error) {
	if err := validateTargetPath(targetPath); err != nil {
		return "", err
	}
	extPath, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		return "", err
	}
	// A symbolic link that resolves into gog's own data directory is
	// bookkeeping: the path is already linked, possibly by another repository,
	// so it is followed. A link to anywhere else belongs to the user, and
	// copying its target would store the contents while discarding the link
	// itself, so the target is named and the link is left alone.
	if isSymlink(targetPath) && !paths.Within(BaseDir, extPath) {
		return "", fmt.Errorf("%q is a symbolic link to %s (add that path instead)", targetPath, extPath)
	}
	if _, err := os.Stat(extPath); err != nil {
		return "", err
	}
	return extPath, nil
}

func isSymlink(p string) bool {
	fileInfo, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink
}

func addPath(repoPath, targetPath string) error {
	extPath, err := resolveAddPath(targetPath)
	if err != nil {
		return err
	}

	intPath := ToInternalPath(repoPath, targetPath)
	if extPath == intPath {
		// Already added
		return nil
	}

	extFileInfo, err := os.Stat(extPath)
	if err != nil {
		return err
	}
	if extFileInfo.IsDir() {
		return copy.Dir(extPath, intPath, shouldSkip)
	}

	// Create the parent directory, because `copy.File` does not create directories
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		return err
	}
	return copy.File(extPath, intPath)
}

func removePath(repoPath, targetPath string) error {
	if err := validateTargetPath(targetPath); err != nil {
		return err
	}
	intPath := ToInternalPath(repoPath, targetPath)
	return os.RemoveAll(intPath)
}

type syncFunc func(string, string) error

// syncRepository synchronizes all given paths within `repoPath`
func syncRepository(repoPath string, targetPaths []string, updateRepository syncFunc) error {
	for _, extPath := range targetPaths {
		if err := updateRepository(repoPath, extPath); err != nil {
			return err
		}
	}
	return nil
}
