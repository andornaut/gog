package repository

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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
			return "", fmt.Errorf("a repository named %q already exists", filepath.Base(repoPath))
		}
		return "", fmt.Errorf("%q already exists and is not a gog repository (remove it or choose another name)", repoPath)
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
		return gitFailure(git.Init(BaseDir, repoPath),
			fmt.Sprintf("failed to initialize a git repository in %s", repoPath))
	}
	return gitFailure(git.Clone(BaseDir, repoPath, repoURL),
		fmt.Sprintf("failed to clone %s", repoURL))
}

// gitFailure names the step of gog's that failed. A wait status is left out:
// git ran and has already explained itself on its own stderr. Any other failure
// means git never ran, and nothing but this error says why.
func gitFailure(err error, step string) error {
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &exitErr):
		return errors.New(step)
	}
	return fmt.Errorf("%s: %w", step, err)
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
// exist only in the repository's own directory. A repository with no remote at
// all reports its whole history.
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
func AddPaths(repoPath string, force bool, targetPaths []string) error {
	for _, targetPath := range targetPaths {
		if _, err := resolveAddPath(repoPath, force, targetPath); err != nil {
			return err
		}
	}
	if err := syncRepository(repoPath, targetPaths, addPath(force)); err != nil {
		return err
	}
	warnUnrecordableModes(repoPath, targetPaths)
	return nil
}

// warnUnrecordableModes reports paths whose permissions git will not carry to
// another machine. Git records only the executable bit, so a path that
// withholds access from group or other is recreated with that access granted:
// a 0600 file becomes 0644 and a 0700 directory becomes 0755. Someone tracking
// ~/.ssh or ~/.netrc would otherwise find it world-readable on the next
// machine, with nothing having reported it.
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

// reportSkipped returns what a copy into repoPath says about an entry it left
// behind. A symbolic link is never followed: copying its target would store the
// contents while discarding the link itself. One that resolves into gog's data
// directory names the repository that manages it.
func reportSkipped(repoPath, resolvedRoot, typedRoot string) copy.ReportFunc {
	return func(p string, mode os.FileMode) {
		p = asTyped(p, resolvedRoot, typedRoot)
		if mode&os.ModeSymlink == 0 {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s %s (git cannot store it)\n", copy.FileKind(mode), p)
			return
		}
		if resolved, err := filepath.EvalSymlinks(p); err == nil && WithinBaseDir(resolved) {
			// This repository's own link, so the path is already added.
			// Advising its removal from here would undo that.
			if paths.Within(paths.Resolve(repoPath), resolved) {
				return
			}
			fmt.Fprintf(os.Stderr, "Warning: skipping %s (repository %s already manages it; remove it from there first)\n",
				p, repoNameOf(resolved))
			return
		}
		if target, err := os.Readlink(p); err == nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping symbolic link %s -> %s (add that path instead)\n", p, target)
			return
		}
		fmt.Fprintf(os.Stderr, "Warning: skipping symbolic link %s (add its target instead)\n", p)
	}
}

// asTyped rewrites a path under resolvedRoot to sit under typedRoot instead, so
// that an entry is named the way the directory it came from was named. The copy
// walks the resolved path, which spells an entry through whatever symbolic
// links the walk took to reach it.
func asTyped(p, resolvedRoot, typedRoot string) string {
	if resolvedRoot == typedRoot {
		return p
	}
	rel, err := filepath.Rel(resolvedRoot, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p
	}
	return filepath.Join(typedRoot, rel)
}

// repoNameOf names the repository that holds p, which must be a path within the
// data directory. Both sides are resolved through their symbolic links: a
// resolved path measured against an unresolved data directory names no
// repository at all.
func repoNameOf(p string) string {
	rel, err := filepath.Rel(paths.Resolve(BaseDir), paths.Resolve(p))
	if err != nil {
		return ""
	}
	name, _, _ := strings.Cut(rel, string(filepath.Separator))
	return name
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
// refuseContentDirCollision stops a repository of the legacy layout from being
// given a path that would create ContentDirName at its top level.
//
// /root is such a path, being where the superuser's home sits. A legacy
// repository stores it at its top level under that name, which is the name that
// decides the layout: the next command would read the repository as one that
// had been moved, look for its content in that directory, and find one file
// there while every other path it holds stopped converting to anything at all.
//
// Refused rather than stored somewhere else, since the repository is one move
// away from having no such collision: under ContentDirName, /root is
// root/root and names nothing but itself.
//
// CLEANUP (added 2026-08-14): goes with the legacy layout, see ContentPath.
func refuseContentDirCollision(repoPath, targetPath, intPath string) error {
	if !legacyLayout(repoPath) {
		return nil
	}
	rel, err := filepath.Rel(repoPath, intPath)
	if err != nil || (rel != ContentDirName && !strings.HasPrefix(rel, ContentDirName+string(filepath.Separator))) {
		return nil
	}
	return fmt.Errorf("refusing to add %q to %s: it would be stored as %s, which is the directory "+
		"a repository keeps its linked content in. Move this repository first: `gog git mv <each top-level directory> %s/`",
		targetPath, filepath.Base(repoPath), rel, ContentDirName)
}

func resolveAddPath(repoPath string, force bool, targetPath string) (string, error) {
	if err := validateTargetPath(targetPath); err != nil {
		return "", err
	}
	extPath, err := filepath.EvalSymlinks(targetPath)
	// A link that resolves into gog's data directory is one gog made, so the
	// path is already managed and the link is followed. A link to anywhere else
	// belongs to the user, and copying its target would store the contents
	// while discarding the link itself, so the target is named instead. This is
	// decided before the resolution error is reported, so that a broken link is
	// named as a link rather than as its missing target.
	if paths.IsSymlink(targetPath) && (err != nil || !WithinBaseDir(extPath)) {
		target, readErr := os.Readlink(targetPath)
		if readErr != nil {
			return "", fmt.Errorf("%q is a symbolic link (add its target instead)", targetPath)
		}
		return "", fmt.Errorf("%q is a symbolic link to %s (add that path instead)", targetPath, target)
	}
	if err != nil {
		return "", describePathError(targetPath, err)
	}
	// Following another repository's link would move the path between
	// repositories, leaving the other holding a copy that nothing points at.
	// A path this repository already holds may be added again.
	if !force && paths.IsSymlink(targetPath) && !paths.Within(paths.Resolve(repoPath), extPath) {
		return "", fmt.Errorf("%q is managed by repository %s (remove it from there first, or pass --force to take it over)",
			targetPath, repoNameOf(extPath))
	}
	info, err := os.Stat(extPath)
	if err != nil {
		return "", describePathError(targetPath, err)
	}
	// A named pipe, socket or device node has no contents to store: git holds
	// neither its kind nor its contents, and opening one to read it blocks
	// until a writer appears or reads without end
	if !info.Mode().IsRegular() && !info.IsDir() {
		return "", fmt.Errorf("%q is a %s (gog manages files and directories)", targetPath, copy.FileKind(info.Mode()))
	}
	return extPath, nil
}

// describePathError states what gog could not do with a path, in the form its
// other failures take. The underlying error names the system call and the
// resolved path it was given, neither of which is what the user typed.
func describePathError(targetPath string, err error) error {
	switch {
	case os.IsNotExist(err):
		return fmt.Errorf("path %q does not exist", targetPath)
	case os.IsPermission(err):
		return fmt.Errorf("cannot read %q: permission denied", targetPath)
	}
	return err
}

// addPath binds --force to the signature that syncRepository calls
func addPath(force bool) syncFunc {
	return func(repoPath, targetPath string) error {
		return addTargetPath(repoPath, force, targetPath)
	}
}

func addTargetPath(repoPath string, force bool, targetPath string) error {
	extPath, err := resolveAddPath(repoPath, force, targetPath)
	if err != nil {
		return err
	}

	intPath := ToInternalPath(repoPath, targetPath)
	if err = refuseContentDirCollision(repoPath, targetPath, intPath); err != nil {
		return err
	}
	// Compared with its symbolic links resolved, as extPath already is: a data
	// directory reached through a symlinked parent spells the same path two
	// ways, and copying a file over itself empties it.
	if extPath == paths.Resolve(intPath) {
		// Already added
		return nil
	}

	extFileInfo, err := os.Stat(extPath)
	if err != nil {
		return describePathError(targetPath, err)
	}

	// Whether the repository already held this path decides what a failed copy
	// can undo
	_, lstatErr := os.Lstat(intPath)
	held := lstatErr == nil

	if extFileInfo.IsDir() {
		err = copy.Dir(extPath, intPath, shouldSkip, reportSkipped(repoPath, extPath, targetPath))
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
// that fails part-way through does not leave files behind that were never
// linked or staged. Only a path the repository did not already hold can be
// discarded: removing one it held would throw away whatever the copy had
// overwritten, so that case is reported instead.
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
			fmt.Fprintf(os.Stderr, "Skipped: %s (not tracked by %s)\n", targetPath, filepath.Base(repoPath))
			return nil
		}
		return err
	}
	// Untracking is not conditional on the external file having been restored:
	// a path whose link the user had already replaced or deleted would keep its
	// copy in the index, and the file would come back on the next machine that
	// applied the repository. --ignore-unmatch covers a path that was never
	// staged.
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
