package repository

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	// BaseDir is the root data directory
	BaseDir string
	homeDir string
)

// GetDefault returns the default repository path
func GetDefault() (string, error) {
	defaultName := os.Getenv("GOG_DEFAULT_REPOSITORY_NAME")
	if defaultName != "" {
		return RootPath(defaultName)
	}
	return getFirst()
}

// List returns a list of repositories
func List() ([]string, error) {
	entries, err := os.ReadDir(BaseDir)
	if err != nil {
		return nil, err
	}
	repoNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		repoName := entry.Name()
		if err := validateRepoName(repoName); err != nil {
			continue
		}
		repoPath := filepath.Join(BaseDir, repoName)
		if err := validateRepoPath(repoPath); err != nil {
			continue
		}
		repoNames = append(repoNames, repoName)
	}
	return repoNames, nil
}

// RootPath returns an absolute filesystem path which corresponds to the given
// repository name or the default repository's path if the given name is empty
func RootPath(name string) (string, error) {
	if name == "" {
		return GetDefault()
	}

	if err := validateRepoName(name); err != nil {
		return "", err
	}
	p := filepath.Join(BaseDir, name)

	// First check if exact match exists
	if err := validateRepoPath(p); err == nil {
		return p, nil
	}

	// Fall back to glob matching only if exact match doesn't exist
	globPaths, err := filepath.Glob(p + "*")
	if err != nil {
		return "", err
	}

	// Validate that we have exactly one match and it's in the correct directory
	if len(globPaths) == 0 {
		return "", fmt.Errorf("repository not found: %s", name)
	}

	// Filter to basenames that start with the requested name
	var validPaths []string
	for _, globPath := range globPaths {
		basename := filepath.Base(globPath)
		if strings.HasPrefix(basename, name) {
			validPaths = append(validPaths, globPath)
		}
	}

	if len(validPaths) == 0 {
		return "", fmt.Errorf("repository not found: %s", name)
	}
	if len(validPaths) > 1 {
		return "", fmt.Errorf("ambiguous repository name %q matches multiple repositories (use a more specific name)", name)
	}

	p = validPaths[0]
	if err := validateRepoPath(p); err != nil {
		return "", err
	}
	return p, nil
}

func getFirst() (string, error) {
	repoNames, err := List()
	if err != nil {
		return "", err
	}
	if len(repoNames) == 0 {
		return "", fmt.Errorf("no valid git repositories found in %s (run `gog repository add` to add one)", BaseDir)
	}
	return filepath.Join(BaseDir, repoNames[0]), nil
}

// getBaseDir returns an absolute, cleaned base directory. Environment
// variables are normalized because trailing slashes or relative paths would
// break path-boundary comparisons and git checks.
func getBaseDir(homeDir string) (string, error) {
	b := os.Getenv("GOG_HOME")
	if b == "" {
		dataDir := os.Getenv("XDG_DATA_HOME")
		if dataDir != "" {
			b = filepath.Join(dataDir, "gog")
		} else {
			b = filepath.Join(homeDir, ".local/share/gog")
		}
	}
	return filepath.Abs(b)
}

func init() {
	var err error

	homeDir, err = os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	// Normalize so that path-boundary comparisons work when $HOME has a
	// trailing slash
	homeDir = filepath.Clean(homeDir)

	BaseDir, err = getBaseDir(homeDir)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(BaseDir, 0755); err != nil {
		log.Fatal(err)
	}
}
