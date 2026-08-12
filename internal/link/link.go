package link

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/paths"
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
	var linked []string
	walkErr := filepath.Walk(intPath, func(p string, info os.FileInfo, err error) error {
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
				ok, backupErr := backup(extPath)
				if !ok {
					printError(p, fmt.Errorf("backup failed, skipping directory: %w", backupErr))
					return filepath.SkipDir
				}
			}

			if mkdirErr := os.MkdirAll(extPath, 0755); mkdirErr != nil {
				printError(p, fmt.Errorf("failed to create directory %s: %w", extPath, mkdirErr))
				return filepath.SkipDir
			}
			return nil
		}
		shouldAdd, linkErr := linkFile(repoPath, p)
		if shouldAdd {
			linked = append(linked, p)
		}
		return linkErr
	})
	// Add the files that were linked, even if the walk failed part-way through
	addToGit(repoPath, linked...)
	return walkErr
}

// File creates a symbolic link from a repository file to the root filesystem.
// File declares an `error` return type to match the signature of `Dir`, but
// usually print an error message and return nil.
func File(repoPath, intPath string) error {
	shouldAdd, err := linkFile(repoPath, intPath)
	if shouldAdd {
		addToGit(repoPath, intPath)
	}
	return err
}

// linkFile creates a symbolic link from a repository file to the root
// filesystem. It returns true if the file is linked and should be added to
// git. It usually prints an error message and returns (false, nil) on failure.
func linkFile(repoPath, intPath string) (bool, error) {
	if ignoreFilesRegex.MatchString(strings.TrimPrefix(intPath, repoPath+"/")) {
		return false, nil
	}
	switch intPath {
	case filepath.Join(repoPath, ".gitignore"):
		return false, nil
	case filepath.Join(repoPath, "LICENSE"):
		return false, nil
	case filepath.Join(repoPath, "README.md"):
		return false, nil
	}

	extPath := repository.ToExternalPath(repoPath, intPath)
	err := os.Symlink(intPath, extPath)
	if err == nil {
		// Success
		printLinked(intPath, extPath)
		return true, nil
	}
	if !os.IsExist(err) {
		// We cannot recover from an error other than extPath already existing, in which case we can back it up.
		return false, fmt.Errorf("failed to create symlink from %s to %s: %w", extPath, intPath, err)
	}

	extFileInfo, err := os.Lstat(extPath)
	if err != nil {
		printError(intPath, fmt.Errorf("failed to stat %s: %w", extPath, err))
		return false, nil
	}
	if extFileInfo.IsDir() {
		printError(intPath, fmt.Errorf("cannot create symlink: %s exists and is a directory (remove the directory or use a different location)", extPath))
		return false, nil
	}

	shouldBackup := !backupDisabled

	// Check if symlink already points to the correct target
	linkTarget, err := os.Readlink(extPath)
	if err == nil && linkTarget == intPath {
		// Already linked to the correct location - no need to recreate
		return true, nil
	}

	// Try to resolve the symlink to check if it's broken
	_, evalErr := filepath.EvalSymlinks(extPath)
	if evalErr != nil {
		// Can only recover from an error due to a broken symbolic link
		if !os.IsNotExist(evalErr) {
			printError(intPath, fmt.Errorf("failed to resolve symlink %s: %w", extPath, evalErr))
			return false, nil
		}
		shouldBackup = false
	}

	// A backup preserves whatever the user had at extPath, so it is pointless
	// when there is nothing of theirs left to preserve. Skipping it keeps a
	// hidden duplicate from accumulating beside every linked file.
	if shouldBackup && (isGogOwnedLink(extPath) || sameContents(extPath, intPath)) {
		shouldBackup = false
	}

	if shouldBackup {
		ok, backupErr := backup(extPath)
		if !ok {
			printError(intPath, fmt.Errorf("backup failed, skipping: %w", backupErr))
			return false, nil
		}
	} else {
		// Either extPath is a broken symbolic link or backups are disabled
		if err = os.Remove(extPath); err != nil {
			printError(intPath, fmt.Errorf("failed to remove %s: %w", extPath, err))
			return false, nil
		}
	}
	if err = os.Symlink(intPath, extPath); err != nil {
		printError(intPath, fmt.Errorf("failed to create symlink from %s to %s: %w", extPath, intPath, err))
		return false, nil
	}
	printLinked(intPath, extPath)
	return true, nil
}

// maxGitAddBatch bounds the number of paths passed to a single git
// invocation in order to stay well under the operating system's argv limit
const maxGitAddBatch = 1000

// addToGit adds the given paths to git in batched invocations. If a batch
// fails, its paths are retried individually so that one bad path does not
// prevent the others from being added.
func addToGit(repoPath string, intPaths ...string) {
	for start := 0; start < len(intPaths); start += maxGitAddBatch {
		batch := intPaths[start:min(start+maxGitAddBatch, len(intPaths))]
		args := append([]string{"add", "--force"}, batch...)
		if err := git.Run(repoPath, args...); err == nil {
			continue
		}
		for _, p := range batch {
			if err := git.Run(repoPath, "add", "--force", p); err != nil {
				printError(p, fmt.Errorf("failed to add %s to git: %w", p, err))
			}
		}
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

// isGogOwnedLink reports whether p is a symbolic link that resolves into gog's
// data directory. Such a link was created by gog itself - by a previous run or
// by another repository that also tracks this path - so it is bookkeeping
// rather than the user's data, and replacing it discards nothing.
func isGogOwnedLink(p string) bool {
	if !isSymlink(p) {
		return false
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return false
	}
	return paths.Within(repository.BaseDir, resolved)
}

// sameContents reports whether both paths are regular files holding identical
// bytes. `add` copies a file into the repository before linking it, so the two
// are identical and backing the original up would only duplicate what the
// repository already holds. Symbolic links are excluded because replacing one
// discards the link itself, which no copy of the contents preserves. Anything
// unreadable or uncertain reports false so that it is backed up rather than
// silently removed.
func sameContents(a, b string) bool {
	aInfo, err := os.Lstat(a)
	if err != nil || !aInfo.Mode().IsRegular() {
		return false
	}
	bInfo, err := os.Lstat(b)
	if err != nil || !bInfo.Mode().IsRegular() {
		return false
	}
	if aInfo.Size() != bInfo.Size() {
		return false
	}
	return equalContents(a, b)
}

func equalContents(a, b string) bool {
	aFile, err := os.Open(a)
	if err != nil {
		return false
	}
	defer func() { _ = aFile.Close() }()

	bFile, err := os.Open(b)
	if err != nil {
		return false
	}
	defer func() { _ = bFile.Close() }()

	aBuf := make([]byte, 64*1024)
	bBuf := make([]byte, 64*1024)
	for {
		aN, aErr := io.ReadFull(aFile, aBuf)
		bN, bErr := io.ReadFull(bFile, bBuf)
		if aN != bN || !bytes.Equal(aBuf[:aN], bBuf[:bN]) {
			return false
		}
		if aErr != nil || bErr != nil {
			// The sizes match, so both files must end together. Any other
			// error leaves the comparison inconclusive.
			return isEnd(aErr) && isEnd(bErr)
		}
	}
}

func isEnd(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
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
