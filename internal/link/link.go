package link

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/paths"
	"github.com/andornaut/gog/internal/repository"
)

// ignoreFilesRegex is what GOG_IGNORE_FILES_REGEX names, and by default
// matches nothing
var ignoreFilesRegex = matchNothing

// matchNothing is a pattern that no path satisfies
var matchNothing = regexp.MustCompile("a^")

// configure reads the environment that linking depends on. Each entry point
// calls it, rather than an init that cannot report a failure and would fail
// every command over a pattern that only the linking commands read.
func configure() error {
	pattern := os.Getenv("GOG_IGNORE_FILES_REGEX")
	if pattern == "" {
		ignoreFilesRegex = matchNothing
		return nil
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid GOG_IGNORE_FILES_REGEX %q: %w", pattern, err)
	}
	ignoreFilesRegex = compiled
	return nil
}

// ErrIncomplete reports that some paths could not be linked. Each failure is
// printed as it happens; this is returned so that the command exits non-zero
// rather than appearing to have succeeded.
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

// Unlink unlinks the given paths
func Unlink(repoPath string, paths []string) error {
	before := failures
	return reportFailures(before, syncLinks(repoPath, paths, UnlinkDir, UnlinkFile))
}

// Link links the given paths
func Link(repoPath string, paths []string) error {
	if err := configure(); err != nil {
		return err
	}
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
	if err := configure(); err != nil {
		return err
	}
	before := failures
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
// File declares an `error` return type to match the signature of `Dir`, but
// usually print an error message and return nil.
func File(repoPath, intPath string) error {
	before := failures
	shouldAdd, err := linkFile(repoPath, intPath)
	if shouldAdd {
		addToGit(repoPath, intPath)
	}
	return reportFailures(before, err)
}

// linkFile creates a symbolic link from a repository file to the root
// filesystem. It returns true if the file is linked and should be added to
// git. It usually prints an error message and returns (false, nil) on failure.
func linkFile(repoPath, intPath string) (bool, error) {
	if skipped(repoPath, intPath) {
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

	// Check if symlink already points to the correct target
	linkTarget, err := os.Readlink(extPath)
	if err == nil && linkTarget == intPath {
		// Already linked to the correct location - no need to recreate
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

// skipped reports whether a path the repository holds is one that is never
// linked: the files a repository keeps for itself, and whatever
// GOG_IGNORE_FILES_REGEX names.
func skipped(repoPath, intPath string) bool {
	if ignoreFilesRegex.MatchString(strings.TrimPrefix(intPath, repoPath+"/")) {
		return true
	}
	switch intPath {
	case filepath.Join(repoPath, ".gitignore"),
		filepath.Join(repoPath, "LICENSE"),
		filepath.Join(repoPath, "README.md"):
		return true
	}
	return false
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
// removed to make way for what the repository holds. A broken link points at
// nothing, and a link into gog's data directory is bookkeeping left by an
// earlier run or by another repository that tracks the same path. Anything else
// is the user's, and gog does not delete it: whatever it holds exists nowhere
// else, unlike the repository's copy.
//
// The error explains why a link could not be resolved, and is only set when the
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
	return fmt.Errorf("%s already exists (move or remove it, then run the command again)", p)
}

// isGogOwnedLink reports whether p is a symbolic link that resolves into gog's
// data directory. Such a link was created by gog itself - by a previous run or
// by another repository that also tracks this path - so it is bookkeeping
// rather than the user's data, and replacing it discards nothing.
func isGogOwnedLink(p string) bool {
	if !paths.IsSymlink(p) {
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
