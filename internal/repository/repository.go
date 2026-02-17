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

	// Filter to only paths that are within BaseDir and start with the expected name
	var validPaths []string
	for _, globPath := range globPaths {
		// Ensure path is within BaseDir (prevent directory traversal)
		if !strings.HasPrefix(globPath, BaseDir+string(filepath.Separator)) {
			continue
		}
		// Ensure the basename starts with the repository name
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
	entries, err := os.ReadDir(BaseDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(BaseDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("run `gog repository add` to add a repository")
}

func getBaseDir(homeDir string) string {
	b := os.Getenv("GOG_HOME")
	if b != "" {
		return b
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir != "" {
		return filepath.Join(dataDir, "gog")
	}

	return filepath.Join(homeDir, ".local/share/gog")
}

func init() {
	var err error

	homeDir, err = os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	BaseDir = getBaseDir(homeDir)
	if err = os.MkdirAll(BaseDir, 0755); err != nil {
		log.Fatal(err)
	}
}
