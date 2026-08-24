package link

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/paths"
	"github.com/andornaut/gog/internal/repository"
)

// ErrIncomplete reports that some paths could not be linked. Each failure is
// printed as it happens; this is returned so that the command exits non-zero.
var ErrIncomplete = errors.New("some paths could not be linked")

// reportFailures returns err if it is set, otherwise ErrIncomplete if anything
// was reported since `before`
func reportFailures(before int, err error) error {
	if err != nil {
		return err
	}
	if failures > before {
		return ErrIncomplete
	}
	return nil
}

// Unlink restores the given external paths as ordinary files
func Unlink(repoPath string, paths []string) error {
	before := failures
	return reportFailures(before, syncLinks(repoPath, paths, UnlinkDir, UnlinkFile))
}

// Link links the given external paths to what the repository holds at them
func Link(repoPath string, paths []string) error {
	before := failures
	return reportFailures(before, syncLinks(repoPath, paths, Dir, File))
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
	// A repository with no content directory holds nothing to link.
	if _, err := os.Stat(intPath); os.IsNotExist(err) {
		return nil
	}
	before := failures
	var linked []string
	walkErr := filepath.Walk(intPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Linked from, not linked. Nothing else needs skipping: the walk starts
		// inside the content directory, and .git sits outside it.
		if p == intPath {
			return nil
		}
		if info.IsDir() {
			extPath := repository.ToExternalPath(repoPath, p)
			if paths.IsSymlink(extPath) {
				// Creating the directory would otherwise write through the link
				// into whatever it points at
				if ok, discardErr := discardable(extPath); !ok {
					printError(refusal(extPath, discardErr))
					return filepath.SkipDir
				}
				if rmErr := os.Remove(extPath); rmErr != nil {
					printError(fmt.Errorf("failed to remove %s: %w", extPath, rmErr))
					return filepath.SkipDir
				}
			}

			if mkdirErr := os.MkdirAll(extPath, 0755); mkdirErr != nil {
				printError(fmt.Errorf("failed to create directory %s: %w", extPath, mkdirErr))
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
	return reportFailures(before, walkErr)
}

// File creates a symbolic link from a repository file to the root filesystem.
// It returns ErrIncomplete when the link could not be made, having printed why.
func File(repoPath, intPath string) error {
	before := failures
	shouldAdd, err := linkFile(repoPath, intPath)
	if shouldAdd {
		addToGit(repoPath, intPath)
	}
	return reportFailures(before, err)
}

// linkFile creates a symbolic link from a repository file to the root
// filesystem, and returns true if the file is linked and should be added to
// git. A failure that leaves the path alone is printed and returns (false,
// nil); only one that stops the run is returned.
func linkFile(repoPath, intPath string) (bool, error) {
	extPath := repository.ToExternalPath(repoPath, intPath)
	err := os.Symlink(intPath, extPath)
	if err == nil {
		printLinked(intPath, extPath)
		return true, nil
	}
	if !os.IsExist(err) {
		// The only recoverable failure is extPath already existing, which is
		// decided below
		return false, fmt.Errorf("failed to create symlink from %s to %s: %w", extPath, intPath, err)
	}

	extFileInfo, err := os.Lstat(extPath)
	if err != nil {
		printError(fmt.Errorf("failed to stat %s: %w", extPath, err))
		return false, nil
	}
	if extFileInfo.IsDir() {
		printError(fmt.Errorf("cannot create symlink: %s exists and is a directory (remove the directory or use a different location)", extPath))
		return false, nil
	}

	// Already linked, so the link is not recreated
	linkTarget, err := os.Readlink(extPath)
	if err == nil && linkTarget == intPath {
		return true, nil
	}

	ok, discardErr := discardable(extPath)
	// `add` copies a path into the repository before linking it, so an
	// identical file is the copy of what is about to be linked
	if !ok && !sameContents(extPath, intPath) {
		printError(refusal(extPath, discardErr))
		return false, nil
	}

	if err = os.Remove(extPath); err != nil {
		printError(fmt.Errorf("failed to remove %s: %w", extPath, err))
		return false, nil
	}
	if err = os.Symlink(intPath, extPath); err != nil {
		printError(fmt.Errorf("failed to create symlink from %s to %s: %w", extPath, intPath, err))
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
				printError(fmt.Errorf("failed to add %s to git: %w", p, err))
			}
		}
	}
}

// discardable reports whether p holds nothing of the user's, so that it can be
// removed to make way for what the repository holds: a broken link points at
// nothing, and a link into gog's data directory was made by gog. Anything else
// is the user's and is not deleted, because what it holds exists nowhere else.
//
// The error explains why a link could not be resolved, and is set only when the
// answer is false.
func discardable(p string) (bool, error) {
	if _, err := filepath.EvalSymlinks(p); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	return isGogOwnedLink(p), nil
}

// refusal reports that a path was left alone, naming the error that decided it
// when there was one
func refusal(p string, err error) error {
	if err != nil {
		return fmt.Errorf("cannot resolve %s, leaving it alone: %w", p, err)
	}
	return fmt.Errorf("%q already exists (move or remove it, then run the command again)", p)
}

// isGogOwnedLink reports whether p is a symbolic link that resolves into gog's
// data directory, which means gog made it: an earlier run, or another
// repository that tracks the same path. Replacing it discards nothing.
func isGogOwnedLink(p string) bool {
	if !paths.IsSymlink(p) {
		return false
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return false
	}
	return repository.WithinBaseDir(resolved)
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
