package cmd

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/link"
	"github.com/andornaut/gog/repository"
)

// Add runs the `gog add` command
func runAdd(repoPath string, paths []string) error {
	paths = cleanPaths(paths)
	if err := updateRepository(repoPath, paths, repository.AddPath); err != nil {
		return err
	}
	return updateLinks(repoPath, paths, link.Dir, link.File)
}

// Remove runs the `gog remove` command
func runRemove(repoPath string, paths []string) error {
	paths = cleanPaths(paths)
	if err := updateLinks(repoPath, paths, link.UnlinkDir, link.UnlinkFile); err != nil {
		return err
	}
	return updateRepository(repoPath, paths, repository.RemovePath)
}

func cleanPaths(paths []string) []string {
	cleanedPaths := []string{}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		p, err := normalizePath(p)
		if err != nil {
			continue
		}
		cleanedPaths = append(cleanedPaths, p)
	}
	return cleanedPaths
}

func normalizePath(p string) (string, error) {
	if !path.IsAbs(p) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = path.Join(cwd, p)
	}

	return filepath.Clean(p), nil
}

type updateLinkFunc func(string, string) error

func updateLinks(repoPath string, paths []string, updateDir, updateFile updateLinkFunc) error {
	for _, extPath := range paths {
		intPath := repository.ToInternalPath(repoPath, extPath)
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

func updateRepository(repoPath string, paths []string, f func(string, string) error) error {
	for _, extPath := range paths {
		if err := f(repoPath, extPath); err != nil {
			return err
		}
	}
	return nil
}
