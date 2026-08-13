package repository

import (
	"fmt"
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
		p, err := RootPath(defaultName)
		if err != nil {
			// The name was never typed on the command line, so naming where it
			// came from is what makes the failure actionable
			return "", fmt.Errorf("%w (named by GOG_DEFAULT_REPOSITORY_NAME)", err)
		}
		return p, nil
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

// fail reports a startup error in the form every other gog failure takes and
// exits. init cannot return one, and a gog that cannot locate its home
// directory or its data directory has nothing left to do.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}
	// Normalize so that path-boundary comparisons work when $HOME has a
	// trailing slash, and so that a relative $HOME is still recognized in the
	// absolute paths gog compares it against. Without this, every path under a
	// relative $HOME is stored by its absolute name instead of under the
	// portable $HOME component, which is the same normalization BaseDir gets.
	home, err = filepath.Abs(home)
	if err != nil {
		fail(err)
	}
	// A home directory that does not exist is a misconfigured environment
	// rather than a new one. Without this check gog creates its data directory
	// under whatever $HOME happens to name and reports success.
	info, statErr := os.Stat(home)
	switch {
	case os.IsNotExist(statErr):
		fail(fmt.Errorf("home directory does not exist: %s", home))
	case statErr != nil:
		fail(statErr)
	case !info.IsDir():
		fail(fmt.Errorf("home directory is not a directory: %s", home))
	}
	homeDir = home

	BaseDir, err = getBaseDir(homeDir)
	if err != nil {
		fail(err)
	}
	if err = os.MkdirAll(BaseDir, 0755); err != nil {
		fail(fmt.Errorf("cannot create gog's data directory: %w", err))
	}
}
