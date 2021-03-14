package link

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/repository"
)

var (
	backupDisabled   = false
	ignoreFilesRegex = regexp.MustCompile("a^") // Do not match anything by default
)

// Unlink unlinks the given paths
func Unlink(repoPath string, paths []string) error {
	return repository.SyncLinks(repoPath, paths, UnlinkDir, UnlinkFile)
}

// Link unlinks the given paths
func Link(repoPath string, paths []string) error {
	return repository.SyncLinks(repoPath, paths, Dir, File)
}

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
		case path.Join(repoPath, ".git"):
			return filepath.SkipDir
		}

		if info.IsDir() {
			extPath := repository.ToExternalPath(repoPath, p)
			if isSymlink(extPath) {
				if ok := backup(extPath); !ok {
					printError(p, errors.New("backup of existing file failed. Skipping"))
					return filepath.SkipDir
				}
			}

			if err := os.MkdirAll(extPath, 0755); err != nil {
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
// usually print an error message and return nil.
func File(repoPath, intPath string) error {
	if ignoreFilesRegex.MatchString(strings.TrimPrefix(intPath, repoPath+"/")) {
		return nil
	}
	switch intPath {
	case path.Join(repoPath, ".gitignore"):
		return nil
	case path.Join(repoPath, "LICENSE"):
		return nil
	case path.Join(repoPath, "README.md"):
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
		printError(intPath, fmt.Errorf("Path expected to be a file, but is a directory: %s", extPath))
		return nil
	}

	shouldBackup := !backupDisabled
	actualExtPath, err := filepath.EvalSymlinks(extPath)
	if err != nil {
		// Can only recover from an error due to a broken symbolic link
		if !os.IsNotExist(err) {
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
			printError(intPath, errors.New("backup of existing file failed. Skipping"))
			return nil
		}
	} else {
		// Either extPath is a broken symbolic link or backups are disabled
		if err = os.Remove(extPath); err != nil {
			printError(intPath, err)
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

func init() {
	_, backupDisabled = os.LookupEnv("GOG_DO_NOT_CREATE_BACKUPS")

	ignoreFilesStr := os.Getenv("GOG_IGNORE_FILES_REGEX")
	if ignoreFilesStr != "" {
		var err error
		ignoreFilesRegex, err = regexp.Compile(ignoreFilesStr)
		if err != nil {
			log.Fatalf("Invalid regular expression GOG_IGNORE_FILES_REGEX: %s\n", ignoreFilesStr)
		}
	}
}
