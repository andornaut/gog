package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/internal/paths"
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
			// The name was never typed on the command line, so the failure
			// names where it came from
			return "", fmt.Errorf("%w (named by GOG_DEFAULT_REPOSITORY_NAME)", err)
		}
		return p, nil
	}
	return getFirst()
}

// basenames returns the repository names of a list of repository paths, in the
// order they were matched.
func basenames(paths []string) []string {
	names := make([]string, 0, len(paths))
	for _, p := range paths {
		names = append(names, filepath.Base(p))
	}
	return names
}

// WithinBaseDir reports whether an already-resolved path lies inside gog's data
// directory. Both sides have to be free of symbolic links: the data directory
// can be reached through a symlinked parent (a temporary directory under /var
// on macOS is one), and a resolved path measured against an unresolved BaseDir
// looks like it lies outside.
func WithinBaseDir(resolved string) bool {
	return paths.Within(paths.Resolve(BaseDir), resolved)
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

	if err := validateRepoPath(p); err == nil {
		return p, nil
	}

	// A name that matches no repository exactly is read as a prefix
	globPaths, err := filepath.Glob(p + "*")
	if err != nil {
		return "", err
	}

	if len(globPaths) == 0 {
		return "", fmt.Errorf("repository %q not found", name)
	}

	// Glob matches by path, so the prefix is checked against the name itself
	var validPaths []string
	for _, globPath := range globPaths {
		basename := filepath.Base(globPath)
		if strings.HasPrefix(basename, name) {
			validPaths = append(validPaths, globPath)
		}
	}

	if len(validPaths) == 0 {
		return "", fmt.Errorf("repository %q not found", name)
	}
	if len(validPaths) > 1 {
		return "", fmt.Errorf("%q begins the name of %d repositories: %s. Use the whole name of the one you mean",
			name, len(validPaths), strings.Join(basenames(validPaths), ", "))
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
		return "", fmt.Errorf("no repositories found in %q (run \"gog repository add\" to add one)", BaseDir)
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

// Configure locates the home directory and the data directory that every
// command works from. It is called once, before any command runs, and returns
// its failures so that they are reported in the same form as every other one.
func Configure() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// Normalized, as BaseDir is, so that path-boundary comparisons hold when
	// $HOME has a trailing slash, and so that a relative $HOME is recognized in
	// the absolute paths gog compares it against rather than storing every path
	// under it by its absolute name
	home, err = filepath.Abs(home)
	if err != nil {
		return err
	}
	// A home directory that does not exist is a misconfigured environment
	// rather than a new one: gog would otherwise create its data directory
	// under whatever $HOME names and report success.
	info, err := os.Stat(home)
	switch {
	case os.IsNotExist(err):
		return fmt.Errorf("home directory %q does not exist", home)
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("home directory is not a directory: %s", home)
	}
	homeDir = home

	BaseDir, err = getBaseDir(homeDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		return fmt.Errorf("cannot create gog's data directory: %w", err)
	}
	return nil
}
