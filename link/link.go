package link

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/repository"
)

// Dir recursively creates symbolic links from a repository directory's files
// to the root filesystem
func Dir(repoPath, intPath string) error {
	return filepath.Walk(intPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		switch p {
		case repoPath:
			return nil
		case path.Join(repoPath, ".gitignore"):
			return nil
		case path.Join(repoPath, ".git"):
			return filepath.SkipDir
		}

		if info.IsDir() {
			extPath := repository.ToExternalPath(repoPath, p)
			if isSymlink(extPath) {
				if ok := backup(extPath); !ok {
					printError(p, "Backup of existing file failed. Skipping.")
					return filepath.SkipDir
				}
			}

			if err := os.MkdirAll(extPath, os.ModePerm); err != nil {
				printError(p, err)
				return filepath.SkipDir
			}
			return nil
		}
		return File(repoPath, p)
	})
}

// File creates a symbolic link from a repository file to the root filesystem.
// File declares an `error` return type to match the signature of `Dir`, but
// always returns nil.
func File(repoPath, intPath string) error {
	if intPath == path.Join(repoPath, "LICENSE") || intPath == path.Join(repoPath, "README.md") {
		return nil
	}
	if !strings.HasSuffix(intPath, "gitignore") {
		return nil
	}

	extPath := repository.ToExternalPath(repoPath, intPath)
	err := os.Symlink(intPath, extPath)
	if err == nil {
		// Success
		printLinked(intPath, extPath)
		return nil
	}
	if !os.IsExist(err) {
		// We cannot recover from an error other than extPath already existing, in which case we can back it up.
		return err
	}

	extFileInfo, err := os.Lstat(extPath)
	if err != nil {
		printError(intPath, err)
		return nil
	}
	if extFileInfo.IsDir() {
		printError(intPath, "Path expected to be a file, but is a directory: "+extPath)
		return nil
	}

	shouldBackup := true
	actualExtPath, err := filepath.EvalSymlinks(extPath)
	if err != nil {
		// Can only recover from an error due to a broken symbolic link
		if !os.IsNotExist(err) {
			printError(intPath, err)
			return nil
		}
		// If extPath is a broken symbolic link, then delete it and continue
		if err = os.Remove(extPath); err != nil {
			printError(intPath, err)
			return nil
		}
		shouldBackup = false
	}

	if actualExtPath == intPath {
		// Already linked
		return nil
	}

	if shouldBackup {
		if ok := backup(extPath); !ok {
			printError(intPath, "Backup of existing file failed. Skipping.")
			return nil
		}
	}

	if err = os.Symlink(intPath, extPath); err != nil {
		printError(intPath, err)
		return nil
	}
	printLinked(intPath, extPath)
	return nil
}

func backup(p string) bool {
	backupPath := backupPath(p)
	if err := os.Rename(p, backupPath); err != nil {
		// It's better to attempt to rename and fail if
		// os.Rename will overwrite existing files, but not existing directories
		return false
	}
	return true
}

func backupPath(p string) string {
	dirname, basename := filepath.Split(p)
	basename = strings.TrimPrefix(basename, ".")
	return path.Join(dirname, fmt.Sprintf(".%s.gog", basename))
}

func isSymlink(p string) bool {
	fileInfo, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink
}
