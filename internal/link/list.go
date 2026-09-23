package link

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/andornaut/gog/internal/paths"
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
	// into gog's data directory, or a copy of what the repository holds
	StateReplace State = "replace"
	// StateConflict is a path holding something of the user's. Applying would
	// report the path and leave it alone.
	StateConflict State = "conflict"
	// StateStale is a link to a file the repository no longer holds, which
	// points at nothing. Applying reports it and leaves it alone.
	StateStale State = "stale"
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
	contentPath := repository.ContentPath(repoPath)
	// A repository with no content directory holds nothing to link.
	if !repository.HasContentDir(repoPath) {
		return nil, nil
	}
	var entries []Entry
	// The repository directories whose external path applying replaces with a
	// real directory. What is under one is missing once it has been replaced,
	// whatever the link it replaces points at.
	// Those it refuses leave what is under them alone, as conflicts.
	var replaced, refused []string
	under := func(dirs []string, p string) bool {
		return slices.ContainsFunc(dirs, func(dir string) bool { return paths.Within(dir, p) })
	}
	err := filepath.Walk(contentPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		extPath := repository.ToExternalPath(repoPath, p)
		if info.IsDir() {
			if p != contentPath && paths.IsSymlink(extPath) {
				switch action, _ := symlinkedDir(extPath); action {
				case replaceDir:
					replaced = append(replaced, p)
				case refuseDir:
					refused = append(refused, p)
				case descendDir:
					// What is under it is decided path by path, as applying does
				}
			}
			return nil
		}
		st := state(repoPath, p, extPath)
		switch {
		case under(refused, p):
			st = StateConflict
		case under(replaced, p):
			st = StateMissing
		}
		entries = append(entries, Entry{ExternalPath: extPath, State: st})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// state reports what applying would do to extPath, by the same tests linkFile
// makes
func state(repoPath, intPath, extPath string) State {
	info, err := os.Lstat(extPath)
	switch {
	case os.IsNotExist(err):
		if !paths.Writable(filepath.Dir(extPath)) {
			return StateConflict
		}
		return StateMissing
	case err != nil:
		return StateConflict
	}
	if repository.LinksTo(extPath, intPath) {
		return StateLinked
	}
	// As applying without --force decides it
	if gogLinkConflict(repoPath, false, extPath) != nil || !paths.Writable(filepath.Dir(extPath)) {
		return StateConflict
	}
	if info.IsDir() {
		if holdsOnlyStaleLinks(repoPath, extPath) {
			return StateReplace
		}
		return StateConflict
	}
	if ok, _ := discardable(extPath); ok || sameContents(extPath, intPath) {
		return StateReplace
	}
	return StateConflict
}
