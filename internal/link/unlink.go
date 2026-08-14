package link

import (
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/copy"
	"github.com/andornaut/gog/internal/repository"
)

// UnlinkDir replaces symbolic links with the files that they linked to. It is
// called with the directory a repository links from, whether that is the whole
// of what it holds or one tree inside it.
func UnlinkDir(repoPath, intPath string) error {
	// A repository being removed before its content directory exists has no
	// link of its own to restore.
	if _, err := os.Stat(intPath); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(intPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		return UnlinkFile(repoPath, p)
	})
}

// UnlinkFile replaces a symbolic link with the file that it linked to, and
// reports success when there is no link of gog's to replace: `gog remove` means
// the same thing whether its link is still there, was replaced with a file of
// the user's, or was deleted, and the caller removes the repository's copy
// either way.
func UnlinkFile(repoPath, intPath string) error {
	extPath := repository.ToExternalPath(repoPath, intPath)

	extFileInfo, err := os.Stat(extPath)
	if err != nil {
		// extPath cannot be examined, so there is no link of gog's to replace
		return nil
	}
	intFileInfo, err := os.Stat(intPath)
	if err != nil {
		return err
	}
	if !os.SameFile(extFileInfo, intFileInfo) {
		// Not this repository's link, so it is left alone
		return nil
	}

	if err := os.Remove(extPath); err != nil {
		return err
	}
	if err := copy.File(intPath, extPath); err != nil {
		return err
	}
	printRestored(extPath)
	return nil
}
