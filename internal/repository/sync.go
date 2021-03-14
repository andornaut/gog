package repository

import (
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/copy"
)

type syncFunc func(string, string) error

// SyncLinks synchronizes all given paths within `repoPath`
func SyncLinks(repoPath string, paths []string, updateDir, updateFile syncFunc) error {
	for _, extPath := range paths {
		intPath := ToInternalPath(repoPath, extPath)
		intFileInfo, err := os.Lstat(intPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Nothing to update
				continue
			}
			return err
		}
		if intFileInfo.IsDir() {
			if err := updateDir(repoPath, intPath); err != nil {
				return err
			}
			continue
		}
		if err := updateFile(repoPath, intPath); err != nil {
			return err
		}
	}
	return nil
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

// syncRepository synchronizes all given paths within `repoPath`
func syncRepository(repoPath string, paths []string, updateRepository syncFunc) error {
	for _, extPath := range paths {
		if err := updateRepository(repoPath, extPath); err != nil {
			return err
		}
	}
	return nil
}
