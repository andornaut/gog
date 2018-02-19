package repository

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/andornaut/gog/copy"
)

// BaseDir is the root data dir, which is usually ~/.local/share/gog
var BaseDir string
var homeDir string

// AddPath adds a path to a repository
func AddPath(repoPath, targetPath string) error {
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
	if err := os.MkdirAll(filepath.Dir(intPath), os.ModePerm); err != nil {
		return err
	}
	return copy.File(extPath, intPath)
}

// RemovePath removes a path from a repository
func RemovePath(repoPath, targetPath string) error {
	if err := validateTargetPath(targetPath); err != nil {
		return err
	}
	intPath := ToInternalPath(repoPath, targetPath)
	return os.RemoveAll(intPath)
}

// GetDefault returns the default repository path
func GetDefault() (string, error) {
	p := os.Getenv("GOG_DEFAULT_REPOSITORY_PATH")
	if p == "" {
		return getFirst()
	}
	if err := validateRepoPath(p); err != nil {
		return "", err
	}
	return p, nil
}

// RootPath returns an absolute filesystem path which corresponds to the given
// repository name or the default repository's path if the given name is empty
func RootPath(name string) (string, error) {
	if name == "" {
		return GetDefault()
	}
	p := path.Join(BaseDir, name)
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

func init() {
	homeDir = os.Getenv("HOME")
	if homeDir == "" {
		log.Fatal("The $HOME environment variable cannot be empty")
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = path.Join(homeDir, ".local/share")
	}

	BaseDir = path.Join(dataDir, "gog")
	if err := os.MkdirAll(BaseDir, 0700); err != nil {
		log.Fatal(err)
	}
}
