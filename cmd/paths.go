package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/internal/repository"
)

// cleanPaths normalizes the paths given to a command. An unusable path fails
// the command rather than being passed over, so that a mistyped argument cannot
// leave the command reporting success for work it never did.
func cleanPaths(paths []string) ([]string, error) {
	cleanedPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			return nil, fmt.Errorf("invalid path %q (a path cannot be empty)", p)
		}
		normalized, err := normalizePath(p)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %w", p, err)
		}
		cleanedPaths = append(cleanedPaths, normalized)
	}
	return cleanedPaths, nil
}

func normalizePath(p string) (string, error) {
	if !path.IsAbs(p) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		p = filepath.Join(cwd, p)
	}

	return filepath.Clean(p), nil
}

func repoPath() (string, error) {
	repoPath, err := repository.RootPath(repositoryFlag)
	if err != nil {
		return "", err
	}
	// Reported on stderr so that stdout carries only what the command produces.
	// `gog git` hands its stdout to git, whose output may be read by a script or
	// redirected to a file.
	fmt.Fprintln(os.Stderr, "Repository:", filepath.Base(repoPath))
	return repoPath, nil
}
