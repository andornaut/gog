package link

import (
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/copy"
	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/repository"
)

// UnlinkDir replaces symbolic links with the files that they linked to
func UnlinkDir(repoPath, intPath string) error {
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

// UnlinkFile replaces a symbolic link with the file that it linked to
func UnlinkFile(repoPath, intPath string) error {
	extPath := repository.ToExternalPath(repoPath, intPath)

	extFileInfo, err := os.Stat(extPath)
	if err != nil {
		// Either `extFile` doesn't exist or there is permission error; in either case it should be skipped
		return nil
	}
	intFileInfo, err := os.Stat(intPath)
	if err != nil {
		return err
	}
	if !os.SameFile(extFileInfo, intFileInfo) {
		// Only update `extPath` if it is a symbolic link to `intPath`
		return nil
	}

	if err := os.Remove(extPath); err != nil {
		return err
	}
	if err := copy.File(intPath, extPath); err != nil {
		return err
	}
	printUnLinked(intPath, extPath)
	return git.Run(repoPath, "rm", "-qf", intPath)
}
