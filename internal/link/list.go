package link

import (
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/repository"
)

// State describes what applying the repository would do to a path. The states
// are decided the way `apply` decides them, so that a listing taken beforehand
// says what the run will do.
type State string

const (
	// StateLinked is a path that gog's link occupies, so applying it again
	// would change nothing
	StateLinked State = "linked"
	// StateMissing is a path with nothing at it, which applying would link
	StateMissing State = "missing"
	// StateReplace is a path holding something that applying discards before
	// linking, because nothing of the user's is lost: a broken link, a link
	// into gog's data directory left by an earlier run or by another repository
	// that tracks the same path, or a copy of what the repository already holds
	StateReplace State = "replace"
	// StateConflict is a path holding something of the user's. Applying would
	// report the path and leave it alone.
	StateConflict State = "conflict"
)

// Entry is one path a repository holds, named as it appears outside the
// repository
type Entry struct {
	ExternalPath string
	State        State
}

// List returns every path the repository would link, in the order a walk of the
// repository meets them. The files that are never linked are left out, so the
// listing is what `apply` would act on rather than what the directory contains.
func List(repoPath string) ([]Entry, error) {
	if err := Configure(); err != nil {
		return nil, err
	}
	var entries []Entry
	err := filepath.Walk(repoPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == filepath.Join(repoPath, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			// A worktree's .git is a file, and skipping a file the way a
			// directory is skipped would skip its siblings as well
			return nil
		}
		if info.IsDir() || skipped(repoPath, p) {
			return nil
		}
		extPath := repository.ToExternalPath(repoPath, p)
		entries = append(entries, Entry{ExternalPath: extPath, State: state(p, extPath)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// state reports what applying would do to extPath, by the same tests linkFile
// makes: a link is measured against the repository's own copy, and anything
// else in the way is measured against what the user would lose.
func state(intPath, extPath string) State {
	if _, err := os.Lstat(extPath); err != nil {
		if os.IsNotExist(err) {
			return StateMissing
		}
		return StateConflict
	}
	if target, err := os.Readlink(extPath); err == nil && target == intPath {
		return StateLinked
	}
	if ok, _ := discardable(extPath); ok || sameContents(extPath, intPath) {
		return StateReplace
	}
	return StateConflict
}
