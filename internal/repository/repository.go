package repository

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
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
	entries, err := ioutil.ReadDir(BaseDir)
	if err != nil {
		return nil, err
	}
	repoNames := []string{}
	for _, fileInfo := range entries {
		repoName := fileInfo.Name()
		if err := validateRepoName(repoName); err != nil {
			continue
		}
		repoPath := path.Join(BaseDir, repoName)
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
	p := path.Join(BaseDir, name)
	globPaths, err := filepath.Glob(p + "*")
	if err != nil {
		return "", err
	}
	if globPaths != nil {
		p = globPaths[0]
	}
	if err := validateRepoPath(p); err != nil {
		return "", err
	}
	return p, nil
}

func getFirst() (string, error) {
	entries, err := ioutil.ReadDir(BaseDir)
	if err != nil {
		return "", err
	}

	for _, fileInfo := range entries {
		if fileInfo.IsDir() {
			return path.Join(BaseDir, fileInfo.Name()), nil
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
		return path.Join(dataDir, "gog")
	}

	return path.Join(homeDir, ".local/share/gog")
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
