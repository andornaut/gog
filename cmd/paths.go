package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/internal/repository"
)

func cleanPaths(paths []string) []string {
	cleanedPaths := []string{}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		normalized, err := normalizePath(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid path %q: %v\n", p, err)
			continue
		}
		cleanedPaths = append(cleanedPaths, normalized)
	}
	return cleanedPaths
}

func normalizePath(p string) (string, error) {
	if !path.IsAbs(p) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		p = path.Join(cwd, p)
	}

	return filepath.Clean(p), nil
}

func repoPath() (string, error) {
	repoPath, err := repository.RootPath(repositoryFlag)
	if err != nil {
		return "", err
	}
	fmt.Println("Repository:", filepath.Base(repoPath))
	return repoPath, nil
}
