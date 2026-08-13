package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/internal/copy"
	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/paths"
)

// Add adds a new repository
func Add(repoName, repoURL string) (string, error) {
	if err := validateRepoName(repoName); err != nil {
		return "", err
	}

	// Reject any existing non-empty path, git repository or not, so that an
	// existing data directory is never converted into a repository. An empty
	// directory may be reused.
	repoPath := filepath.Join(BaseDir, repoName)
	entries, err := os.ReadDir(repoPath)
	switch {
	case err == nil && len(entries) > 0:
		// Distinguish an already-configured repository from an unrelated
		// directory so the user is not told to remove real data
		if git.Is(repoPath) {
			return "", fmt.Errorf("repository already exists: %s", repoPath)
		}
		return "", fmt.Errorf("path already exists and is not a gog repository: %s (remove it or choose another name)", repoPath)
	case err != nil && !os.IsNotExist(err):
		return "", fmt.Errorf("invalid repository path %s: %w", repoPath, err)
	}

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", err
	}

	if err := initRepo(repoPath, repoURL); err != nil {
		// Leave nothing in the data directory that is not a repository: a
		// failed clone otherwise leaves the directory it was cloning into,
		// empty or half written
		if rmErr := os.RemoveAll(repoPath); rmErr != nil {
			return "", fmt.Errorf("%w (and %s could not be removed: %v)", err, repoPath, rmErr)
		}
		return "", err
	}
	return repoPath, nil
}

// initRepo creates the git repository at repoPath, by cloning repoURL if one
// was given and initializing an empty repository otherwise
func initRepo(repoPath, repoURL string) error {
	if repoURL == "" {
		return git.Init(BaseDir, repoPath)
	}
	return git.Clone(BaseDir, repoPath, repoURL)
}

// RemovalPath returns the path of the repository with the given name. Unlike
// RootPath it does not accept a prefix: a name that selects a repository for
// deletion is given in full, so that a short one cannot match something the
// user did not mean. The name is validated first because an empty one would
// otherwise be read as a request for the default repository, which must never
// be deleted by omission.
func RemovalPath(repoName string) (string, error) {
	if err := validateRepoName(repoName); err != nil {
		return "", err
	}
	repoPath := filepath.Join(BaseDir, repoName)
	if err := validateRepoPath(repoPath); err != nil {
		return "", err
	}
	return repoPath, nil
}

// Remove deletes the repository at the given path
func Remove(repoPath string) error {
	return os.RemoveAll(repoPath)
}

// UnsavedWork describes what deleting the given repository would destroy:
// commits that no remote holds, and changes that were never committed. Both
// exist only in the repository's own directory, which makes its deletion the
// one thing gog does that cloning again cannot undo. A repository with no
// remote at all reports its whole history, because that is exactly what is
// held nowhere else.
func UnsavedWork(repoPath string) ([]string, error) {
	unpushed, err := git.Output(repoPath, "log", "--oneline", "--branches", "--not", "--remotes")
	if err != nil {
		return nil, err
	}
	uncommitted, err := git.Output(repoPath, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	var unsaved []string
	if n := countLines(unpushed); n > 0 {
		unsaved = append(unsaved, fmt.Sprintf("%s that no remote has", quantify(n, "commit")))
	}
	if n := countLines(uncommitted); n > 0 {
		unsaved = append(unsaved, quantify(n, "uncommitted change"))
	}
	return unsaved, nil
}

func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func quantify(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// AddPaths adds the given paths from the given repository. Every path is
// checked before any is copied, so that one unusable path fails the command
// outright instead of leaving the repository holding files that were never
// linked or staged.
func AddPaths(repoPath string, targetPaths []string) error {
	for _, targetPath := range targetPaths {
		if _, err := resolveAddPath(targetPath); err != nil {
			return err
		}
	}
	if err := syncRepository(repoPath, targetPaths, addPath); err != nil {
		return err
	}
	warnUnrecordableModes(repoPath, targetPaths)
	return nil
}

// warnUnrecordableModes reports paths whose permissions git will not carry to
// another machine. Git records only the executable bit, so a path that
// withholds access from group or other is recreated with that access granted:
// a 0600 file becomes 0644 and a 0700 directory becomes 0755. Someone tracking
// ~/.ssh or ~/.netrc would otherwise find their secrets world-readable on the
// next machine, with nothing having reported it.
func warnUnrecordableModes(repoPath string, targetPaths []string) {
	for _, targetPath := range targetPaths {
		intPath := ToInternalPath(repoPath, targetPath)
		// Walk errors are ignored: the paths were just written, and a warning
		// must never turn a successful add into a failure
		_ = filepath.Walk(intPath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			recorded, widened := widenedMode(info.Mode())
			if !widened {
				return nil
			}
			fmt.Fprintf(os.Stderr,
				"Warning: %s has mode %04o, which git does not record; it will be applied as %04o on another machine\n",
				ToExternalPath(repoPath, p), info.Mode().Perm(), recorded)
			return nil
		})
	}
}

// widenedMode returns the permissions a path will be given on another machine
// and whether that grants access the current mode withholds. A mode that is
// more permissive than the recorded one is merely tightened elsewhere, which
// costs nothing; one that is more restrictive is silently widened, which
// exposes the contents.
func widenedMode(mode os.FileMode) (os.FileMode, bool) {
	perm := mode.Perm()
	if mode.IsDir() {
		// Git does not track directories at all, so they are created by
		// whoever needs them, which for gog means 0755
		return 0755, perm&0755 != 0755
	}
	if !mode.IsRegular() {
		return perm, false
	}
	recorded := os.FileMode(0644)
	if perm&0111 != 0 {
		recorded = 0755
	}
	return recorded, perm&recorded != recorded
}

// RemovePaths removes the given paths from the given repository
func RemovePaths(repoPath string, targetPaths []string) error {
	return syncRepository(repoPath, targetPaths, removePath)
}

// ValidateTargetPaths returns an error if any of the given paths is one gog
// must not manage. `gog remove` checks the whole batch before it restores
// anything, so that an unusable path fails the command outright instead of
// leaving the paths before it half-restored.
func ValidateTargetPaths(targetPaths []string) error {
	for _, targetPath := range targetPaths {
		if err := validateTargetPath(targetPath); err != nil {
			return err
		}
	}
	return nil
}

// resolveAddPath validates a path given to `gog add` and returns the path
// whose contents will be copied into the repository
func resolveAddPath(targetPath string) (string, error) {
	if err := validateTargetPath(targetPath); err != nil {
		return "", err
	}
	extPath, err := filepath.EvalSymlinks(targetPath)
	// A symbolic link that resolves into gog's own data directory is
	// bookkeeping: the path is already linked, possibly by another repository,
	// so it is followed. A link to anywhere else belongs to the user, and
	// copying its target would store the contents while discarding the link
	// itself, so the target is named and the link is left alone. This is
	// decided before the resolution error is reported, so that a broken link
	// is named as a link rather than as its missing target.
	if isSymlink(targetPath) && (err != nil || !paths.Within(BaseDir, extPath)) {
		target, readErr := os.Readlink(targetPath)
		if readErr != nil {
			return "", fmt.Errorf("%q is a symbolic link (add its target instead)", targetPath)
		}
		return "", fmt.Errorf("%q is a symbolic link to %s (add that path instead)", targetPath, target)
	}
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(extPath); err != nil {
		return "", err
	}
	return extPath, nil
}

func isSymlink(p string) bool {
	fileInfo, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink
}

func addPath(repoPath, targetPath string) error {
	extPath, err := resolveAddPath(targetPath)
	if err != nil {
		return err
	}

	intPath := ToInternalPath(repoPath, targetPath)
	if extPath == intPath {
		// Already added
		return nil
	}

	extFileInfo, err := os.Stat(extPath)
	if err != nil {
		return err
	}

	// Whether the repository already held this path decides what a failed copy
	// can undo
	_, lstatErr := os.Lstat(intPath)
	held := lstatErr == nil

	if extFileInfo.IsDir() {
		err = copy.Dir(extPath, intPath, shouldSkip)
	} else if err = os.MkdirAll(filepath.Dir(intPath), 0755); err == nil {
		// The parent directory is created here, because `copy.File` does not
		// create directories
		err = copy.File(extPath, intPath)
	}
	if err != nil {
		return undoCopy(repoPath, intPath, held, err)
	}
	return nil
}

// undoCopy discards what a failed copy left in the repository, so that a tree
// that fails part-way through - on a file that cannot be read, say - does not
// leave files behind that were never linked or staged. Only a path the
// repository did not already hold can be discarded: removing one it held would
// throw away whatever the copy had overwritten, which nothing has a record of,
// so that case is reported instead.
func undoCopy(repoPath, intPath string, held bool, cause error) error {
	if held {
		return fmt.Errorf("%w (%s still holds a partial copy of %s; rerun to complete it)",
			cause, filepath.Base(repoPath), ToExternalPath(repoPath, intPath))
	}
	if err := os.RemoveAll(intPath); err != nil {
		return fmt.Errorf("%w (and %s could not be discarded: %v)", cause, intPath, err)
	}
	return cause
}

func removePath(repoPath, targetPath string) error {
	if err := validateTargetPath(targetPath); err != nil {
		return err
	}
	intPath := ToInternalPath(repoPath, targetPath)
	if _, err := os.Lstat(intPath); err != nil {
		if os.IsNotExist(err) {
			// Reported rather than passed over in silence, because a path this
			// repository never held looks exactly like one it just gave back
			fmt.Printf("Not tracked by %s: %s\n", filepath.Base(repoPath), targetPath)
			return nil
		}
		return err
	}
	// Untracking is not conditional on the external file having been restored.
	// It once lived alongside that restoration, so removing a path whose link
	// the user had already replaced or deleted left the copy in the index, and
	// the file came back on the next machine that applied the repository.
	// --ignore-unmatch covers a path that was never staged.
	if err := git.Run(repoPath, "rm", "-q", "-f", "-r", "--ignore-unmatch", intPath); err != nil {
		return err
	}
	// `git rm` has already deleted what it tracked; this clears anything left,
	// such as a file that was only ever copied in
	return os.RemoveAll(intPath)
}

type syncFunc func(string, string) error

// syncRepository synchronizes all given paths within `repoPath`
func syncRepository(repoPath string, targetPaths []string, updateRepository syncFunc) error {
	for _, extPath := range targetPaths {
		if err := updateRepository(repoPath, extPath); err != nil {
			return err
		}
	}
	return nil
}
