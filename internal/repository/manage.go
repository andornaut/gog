package repository

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/andornaut/gog/internal/copy"
	"github.com/andornaut/gog/internal/git"
)

// Add adds a new repository
func Add(repoName, repoURL string) (string, error) {
	if err := validateRepoName(repoName); err != nil {
		return "", err
	}

	repoPath := path.Join(BaseDir, repoName)
	if err := validateRepoPath(repoPath); err == nil {
		return "", fmt.Errorf("repository already exists: %s", repoPath)
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

// Remove removes an existing repository
func Remove(repoName string) (string, error) {
	if err := validateRepoName(repoName); err != nil {
		return "", err
	}
	repoPath := path.Join(BaseDir, repoName)
	if err := validateRepoPath(repoPath); err != nil {
		return "", err
	}
	if err := os.RemoveAll(repoPath); err != nil {
		return "", err
	}
	return repoPath, nil
}

// AddPaths adds the given paths from the given repository
func AddPaths(repoPath string, paths []string) error {
	return syncRepository(repoPath, paths, addPath)
}

// RemovePaths removes the given paths from the given repository
func RemovePaths(repoPath string, paths []string) error {
	return syncRepository(repoPath, paths, removePath)
}

func addPath(repoPath, targetPath string) error {
	if err := validateTargetPath(targetPath); err != nil {
		return err
	}
	extPath, err := filepath.EvalSymlinks(targetPath)
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
func syncRepository(repoPath string, paths []string, updateRepository syncFunc) error {
	for _, extPath := range paths {
		if err := updateRepository(repoPath, extPath); err != nil {
			return err
		}
	}
	return nil
}
