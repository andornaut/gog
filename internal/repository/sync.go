package repository

import (
	"os"
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

// SyncRepository synchronizes all given paths within `repoPath`
func SyncRepository(repoPath string, paths []string, updateRepository syncFunc) error {
	for _, extPath := range paths {
		if err := updateRepository(repoPath, extPath); err != nil {
			return err
		}
	}
	return nil
}
