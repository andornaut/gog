package link

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/fscopy"
	"github.com/andornaut/gog/internal/paths"
	"github.com/andornaut/gog/internal/repository"
)

// UnlinkDir replaces symbolic links with the files that they linked to, given
// the content directory or one tree inside it. A path with nothing at it is left
// that way: a repository being deleted may never have been applied here.
func UnlinkDir(repoPath, intPath string) error {
	return unlinkDir(repoPath, intPath, false)
}

// UnlinkFile replaces this repository's link at the path intPath is linked from
// with what intPath holds. A path with nothing at it is left that way.
func UnlinkFile(repoPath, intPath string) error {
	return unlinkFile(repoPath, intPath, false)
}

func unlinkDir(repoPath, intPath string, restoreMissing bool) error {
	// Nothing there is nothing to restore.
	if _, err := os.Lstat(intPath); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(intPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return unlinkFile(repoPath, p, restoreMissing)
	})
}

// unlinkFile restores what intPath holds at the path it is linked from, when
// that path is this repository's link to it. The caller removes the
// repository's copy afterwards, so anything that would leave the user without
// the file is an error rather than a path passed over:
//
//   - A path reached through a symbolic link into gog's data directory is the
//     repository's copy itself.
//   - A path that cannot be examined may hold the link.
//   - With restoreMissing, a path with nothing at it is given the file back:
//     `gog rm` names the path, and the repository's copy is its only one.
//
// A path holding anything else, such as a file of the user's or another
// repository's link, is the user's, and is left alone.
func unlinkFile(repoPath, intPath string, restoreMissing bool) error {
	extPath := repository.ToExternalPath(repoPath, intPath)

	if repository.WithinBaseDir(paths.ResolveParent(extPath)) {
		return fmt.Errorf("cannot restore %s: it is reached through a symbolic link into gog's data directory (replace that link with a directory first)", extPath)
	}

	_, err := os.Lstat(extPath)
	switch {
	case os.IsNotExist(err):
		if !restoreMissing {
			return nil
		}
		if mkdirErr := os.MkdirAll(filepath.Dir(extPath), 0755); mkdirErr != nil {
			return fmt.Errorf("cannot restore %s: %w", extPath, mkdirErr)
		}
	case err != nil:
		return fmt.Errorf("cannot restore %s: %w", extPath, err)
	case !repository.LinksTo(extPath, intPath):
		return nil
	}

	if err := restore(intPath, extPath); err != nil {
		return fmt.Errorf("cannot restore %s: %w", extPath, err)
	}
	printRestored(extPath)
	return nil
}

// restore puts what intPath holds at extPath, replacing whatever is there in
// one rename. A symbolic link that the repository holds is restored as that
// link: copying through it would store its target's contents, and the target
// may be a file of the user's.
func restore(intPath, extPath string) error {
	info, err := os.Lstat(intPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fscopy.File(intPath, extPath)
	}
	target, err := os.Readlink(intPath)
	if err != nil {
		return err
	}
	return replaceWithSymlink(target, extPath)
}

// replaceWithSymlink makes p a symbolic link to target, replacing whatever is
// at p in one rename
func replaceWithSymlink(target, p string) error {
	tmp, err := os.CreateTemp(filepath.Dir(p), ".gog-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	// The name was only reserved: a symbolic link cannot be created over it
	if err := os.Remove(name); err != nil {
		return err
	}
	if err := os.Symlink(target, name); err != nil {
		return err
	}
	if err := os.Rename(name, p); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
