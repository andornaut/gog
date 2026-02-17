package link

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/repository"
)

var (
	backupDisabled   = false
	ignoreFilesRegex = regexp.MustCompile("a^") // Do not match anything by default
)

// Unlink unlinks the given paths
func Unlink(repoPath string, paths []string) error {
	return syncLinks(repoPath, paths, UnlinkDir, UnlinkFile)
}

// Link unlinks the given paths
func Link(repoPath string, paths []string) error {
	return syncLinks(repoPath, paths, Dir, File)
}

type syncFunc func(string, string) error

func syncLinks(repoPath string, paths []string, updateDir, updateFile syncFunc) error {
	for _, extPath := range paths {
		intPath := repository.ToInternalPath(repoPath, extPath)
		intFileInfo, err := os.Lstat(intPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Nothing to update
				continue
			}
			return err
		}
		if intFileInfo.IsDir() {
			if err := updateDir(repoPath, intPath); err != nil {
				return err
			}
			continue
		}
		if err := updateFile(repoPath, intPath); err != nil {
			return err
		}
	}
	return nil
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
		case filepath.Join(repoPath, ".git"):
			return filepath.SkipDir
		}

		if info.IsDir() {
			extPath := repository.ToExternalPath(repoPath, p)
			if isSymlink(extPath) {
				ok, err := backup(extPath)
				if !ok {
					printError(p, fmt.Errorf("backup failed, skipping directory: %w", err))
					return filepath.SkipDir
				}
			}

			if err := os.MkdirAll(extPath, 0755); err != nil {
				printError(p, fmt.Errorf("failed to create directory %s: %w", extPath, err))
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
	case filepath.Join(repoPath, ".gitignore"):
		return nil
	case filepath.Join(repoPath, "LICENSE"):
		return nil
	case filepath.Join(repoPath, "README.md"):
		return nil
	}

	extPath := repository.ToExternalPath(repoPath, intPath)
	err := os.Symlink(intPath, extPath)
	if err == nil {
		// Success
		printLinked(intPath, extPath)
		addToGit(repoPath, intPath)
		return nil
	}
	if !os.IsExist(err) {
		// We cannot recover from an error other than extPath already existing, in which case we can back it up.
		return fmt.Errorf("failed to create symlink from %s to %s: %w", extPath, intPath, err)
	}

	extFileInfo, err := os.Lstat(extPath)
	if err != nil {
		printError(intPath, fmt.Errorf("failed to stat %s: %w", extPath, err))
		return nil
	}
	if extFileInfo.IsDir() {
		printError(intPath, fmt.Errorf("cannot create symlink: %s exists and is a directory (remove the directory or use a different location)", extPath))
		return nil
	}

	shouldBackup := !backupDisabled

	// Check if symlink already points to the correct target
	linkTarget, err := os.Readlink(extPath)
	if err == nil && linkTarget == intPath {
		// Already linked to the correct location - no need to recreate
		addToGit(repoPath, intPath)
		return nil
	}

	// Try to resolve the symlink to check if it's broken
	_, evalErr := filepath.EvalSymlinks(extPath)
	if evalErr != nil {
		// Can only recover from an error due to a broken symbolic link
		if !os.IsNotExist(evalErr) {
			printError(intPath, fmt.Errorf("failed to resolve symlink %s: %w", extPath, evalErr))
			return nil
		}
		shouldBackup = false
	}

	if shouldBackup {
		ok, backupErr := backup(extPath)
		if !ok {
			printError(intPath, fmt.Errorf("backup failed, skipping: %w", backupErr))
			return nil
		}
	} else {
		// Either extPath is a broken symbolic link or backups are disabled
		if err = os.Remove(extPath); err != nil {
			printError(intPath, fmt.Errorf("failed to remove %s: %w", extPath, err))
			return nil
		}
	}
	if err = os.Symlink(intPath, extPath); err != nil {
		printError(intPath, fmt.Errorf("failed to create symlink from %s to %s: %w", extPath, intPath, err))
		return nil
	}
	printLinked(intPath, extPath)
	addToGit(repoPath, intPath)
	return nil
}

func addToGit(repoPath, intPath string) {
	if err := git.Run(repoPath, "add", "--force", intPath); err != nil {
		printError(intPath, fmt.Errorf("failed to add %s to git: %w", intPath, err))
	}
}

func backup(p string) (bool, error) {
	backupPath := backupPath(p)
	if err := os.Rename(p, backupPath); err != nil {
		// It's better to attempt to rename and fail if
		// os.Rename will overwrite existing files, but not existing directories
		return false, fmt.Errorf("failed to rename %s to %s: %w", p, backupPath, err)
	}
	return true, nil
}

func backupPath(p string) string {
	dirname, basename := filepath.Split(p)
	basename = strings.TrimPrefix(basename, ".")
	return filepath.Join(dirname, fmt.Sprintf(".%s.gog", basename))
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
