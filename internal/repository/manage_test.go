package repository

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andornaut/gog/internal/gittest"
	"github.com/andornaut/gog/internal/testout"
)

// newSandbox creates a data directory holding one repository, and a home
// directory to add paths from, and points the package at both for the duration
// of the test
func newSandbox(t *testing.T) (repoPath, homeDir string) {
	t.Helper()
	root := t.TempDir()
	baseDir := filepath.Join(root, "data")
	repoPath = filepath.Join(baseDir, "dots")
	homeDir = filepath.Join(root, "home")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatal(err)
	}
	gittest.Init(t, repoPath)
	gittest.Isolate(t, homeDir)

	// Whatever the developer has set would otherwise choose the repository
	t.Setenv("GOG_DEFAULT_REPOSITORY_NAME", "")

	originalBase := BaseDir
	BaseDir = baseDir
	originalHome := SetHomeDirForTest(homeDir)
	t.Cleanup(func() {
		BaseDir = originalBase
		SetHomeDirForTest(originalHome)
	})
	return repoPath, homeDir
}

func writeFile(t *testing.T, p, contents string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func symlink(t *testing.T, target, p string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	return p
}

// A path in the home directory is stored under a literal $HOME component, which
// is what makes a repository portable between machines
func TestAddPathsStoresHomePathsPortably(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	target := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")

	if err := AddPaths(repoPath, false, []string{target}); err != nil {
		t.Fatalf("AddPaths() = %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc"))
	if err != nil || string(contents) != "bashrc\n" {
		t.Errorf("the repository holds %q (%v), want the file's contents", contents, err)
	}
}

func TestAddPathsCopiesADirectoryTree(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	writeFile(t, filepath.Join(homeDir, ".config/app/conf"), "conf\n")
	writeFile(t, filepath.Join(homeDir, ".config/app/nested/deep.conf"), "deep\n")

	if err := AddPaths(repoPath, false, []string{filepath.Join(homeDir, ".config")}); err != nil {
		t.Fatalf("AddPaths() = %v", err)
	}

	for _, rel := range []string{"$HOME/.config/app/conf", "$HOME/.config/app/nested/deep.conf"} {
		if _, err := os.Stat(filepath.Join(repoPath, ContentDirName, rel)); err != nil {
			t.Errorf("the repository does not hold %s: %v", rel, err)
		}
	}
}

// gog's data directory sits under the home directory by default, so adding one
// of its ancestors would copy every repository into the one being added to
func TestAddPathsLeavesOutGogsOwnDataDirectory(t *testing.T) {
	_, homeDir := newSandbox(t)
	// Restored by the sandbox's own cleanup
	BaseDir = filepath.Join(homeDir, ".local", "share", "gog")
	repoPath := filepath.Join(BaseDir, "dots")
	gittest.Init(t, repoPath)
	writeFile(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".vimrc"), "held\n")
	writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")

	if err := AddPaths(repoPath, false, []string{homeDir}); err != nil {
		t.Fatalf("AddPaths() = %v", err)
	}

	held := filepath.Join(repoPath, ContentDirName, "$HOME")
	if _, err := os.Lstat(filepath.Join(held, ".local")); !os.IsNotExist(err) {
		t.Errorf("the repository holds gog's data directory (%v)", err)
	}
	if _, err := os.Stat(filepath.Join(held, ".bashrc")); err != nil {
		t.Errorf("the rest of the directory was not added: %v", err)
	}
}

// The whole batch is checked before anything is copied, so that one unusable
// path does not leave the repository holding files that were never linked
func TestAddPathsChecksEveryPathBeforeCopyingAny(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	good := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")
	missing := filepath.Join(homeDir, ".gone")

	err := AddPaths(repoPath, false, []string{good, missing})

	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("AddPaths() = %v, want a failure naming the missing path", err)
	}
	if _, statErr := os.Stat(filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc")); !os.IsNotExist(statErr) {
		t.Error("the repository holds the first path although the batch failed")
	}
}

// What gog refuses to manage, and how it says so
func TestAddPathsRefusals(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, homeDir string) string
		want    string
	}{
		{
			name: "a symbolic link names its target instead",
			prepare: func(t *testing.T, homeDir string) string {
				t.Helper()
				target := writeFile(t, filepath.Join(homeDir, "real.conf"), "real\n")
				link := filepath.Join(homeDir, ".link")
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				return link
			},
			want: "is a symbolic link",
		},
		{
			// Decided before the resolution error is reported, or the failure
			// would name the missing target rather than the link
			name: "a broken symbolic link is still a link",
			prepare: func(t *testing.T, homeDir string) string {
				t.Helper()
				p := filepath.Join(homeDir, ".broken")
				if err := os.Symlink(filepath.Join(homeDir, "gone"), p); err != nil {
					t.Fatal(err)
				}
				return p
			},
			want: "is a symbolic link to ",
		},
		{
			name: "a named pipe has nothing git can store",
			prepare: func(t *testing.T, homeDir string) string {
				t.Helper()
				p := filepath.Join(homeDir, ".pipe")
				if out, err := exec.Command("mkfifo", p).CombinedOutput(); err != nil {
					t.Skipf("mkfifo unavailable: %v: %s", err, out)
				}
				return p
			},
			want: "is a named pipe",
		},
		{
			name: "a path that does not exist",
			prepare: func(t *testing.T, homeDir string) string {
				t.Helper()
				return filepath.Join(homeDir, ".gone")
			},
			want: "does not exist",
		},
		{
			name: "a path inside a repository",
			prepare: func(t *testing.T, _ string) string {
				t.Helper()
				return filepath.Join(BaseDir, "dots", ContentDirName, "$HOME", ".bashrc")
			},
			want: "repository dots holds it",
		},
		{
			name: "a backup left by an older version",
			prepare: func(t *testing.T, homeDir string) string {
				t.Helper()
				return writeFile(t, filepath.Join(homeDir, ".bashrc.gog"), "old\n")
			},
			want: ".gog backup files cannot be managed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, homeDir := newSandbox(t)
			target := tt.prepare(t, homeDir)

			err := AddPaths(repoPath, false, []string{target})

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("AddPaths() = %v, want a failure containing %q", err, tt.want)
			}
		})
	}
}

// Following another repository's link would move the path between
// repositories, so it is refused unless --force says to take it over
func TestAddPathsRefusesAPathAnotherRepositoryManages(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	managed := writeFile(t, filepath.Join(BaseDir, "other", ContentDirName, "$HOME", ".bashrc"), "theirs\n")
	target := symlink(t, managed, filepath.Join(homeDir, ".bashrc"))

	err := AddPaths(repoPath, false, []string{target})

	want := "is managed by repository other (remove it from there first, or pass --force to take it over)"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("AddPaths() = %v, want a failure containing %q", err, want)
	}
	if _, statErr := os.Lstat(filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc")); !os.IsNotExist(statErr) {
		t.Error("the repository took the path although it refused")
	}

	// --force takes it over
	if err = AddPaths(repoPath, true, []string{target}); err != nil {
		t.Fatalf("AddPaths(force) = %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc"))
	if err != nil || string(contents) != "theirs\n" {
		t.Errorf("the repository holds %q (%v), want the contents it took over", contents, err)
	}
}

// A path this repository already holds may be added again
func TestAddPathsFollowsItsOwnLink(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	held := writeFile(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc"), "bashrc\n")
	target := symlink(t, held, filepath.Join(homeDir, ".bashrc"))

	if err := AddPaths(repoPath, false, []string{target}); err != nil {
		t.Fatalf("AddPaths() = %v", err)
	}

	contents, err := os.ReadFile(held)
	if err != nil || string(contents) != "bashrc\n" {
		t.Errorf("the repository holds %q (%v), want what it held", contents, err)
	}
}

// A path inside the data directory is refused by the repository that holds it,
// and by the path it is linked from
func TestOwnPathError(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	linked := writeFile(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc"), "bashrc\n")
	symlink(t, linked, filepath.Join(homeDir, ".bashrc"))
	unlinked := writeFile(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".vimrc"), "vimrc\n")

	tests := []struct {
		name string
		p    string
		want string
	}{
		{
			name: "the data directory itself",
			p:    BaseDir,
			want: "gog's own data directory cannot be managed",
		},
		{
			name: "a repository",
			p:    repoPath,
			want: "that is repository dots; name the paths it holds instead",
		},
		{
			name: "a path it holds and has linked",
			p:    linked,
			want: "repository dots holds it; name " + filepath.Join(homeDir, ".bashrc") + " instead",
		},
		{
			name: "a path it holds that is linked nowhere",
			p:    unlinked,
			want: "repository dots holds it",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTargetPath(tt.p)

			if err == nil || !strings.HasSuffix(err.Error(), "("+tt.want+")") {
				t.Errorf("validateTargetPath() = %v, want it to end with %q", err, tt.want)
			}
		})
	}
}

// A copy walks the resolved path, so an entry is named the way the directory it
// came from was named
func TestAsTyped(t *testing.T) {
	tests := []struct {
		name         string
		p            string
		resolvedRoot string
		typedRoot    string
		want         string
	}{
		{name: "under the resolved root", p: "/real/conf/one", resolvedRoot: "/real/conf", typedRoot: "/home/conf", want: "/home/conf/one"},
		{name: "the root itself", p: "/real/conf", resolvedRoot: "/real/conf", typedRoot: "/home/conf", want: "/home/conf"},
		{name: "roots that are the same", p: "/real/conf/one", resolvedRoot: "/real/conf", typedRoot: "/real/conf", want: "/real/conf/one"},
		{name: "outside the resolved root", p: "/real/other/one", resolvedRoot: "/real/conf", typedRoot: "/home/conf", want: "/real/other/one"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := asTyped(tt.p, tt.resolvedRoot, tt.typedRoot); got != tt.want {
				t.Errorf("asTyped(%q) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

// `gog remove` checks the whole batch before it restores anything
func TestValidateTargetPaths(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	mine := filepath.Join(homeDir, ".bashrc")

	if err := ValidateTargetPaths([]string{mine}); err != nil {
		t.Errorf("ValidateTargetPaths() = %v, want success", err)
	}

	err := ValidateTargetPaths([]string{mine, filepath.Join(repoPath, ContentDirName, "$HOME", ".vimrc")})

	if err == nil || !strings.Contains(err.Error(), "repository dots holds it") {
		t.Errorf("ValidateTargetPaths() = %v, want the batch refused", err)
	}
}

// What a copy leaves behind is named with what to do about it. A link into
// gog's data directory names the repository that manages it.
func TestReportSkipped(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	managed := writeFile(t, filepath.Join(BaseDir, "other", ContentDirName, "$HOME", ".bashrc"), "bashrc\n")
	elsewhere := writeFile(t, filepath.Join(homeDir, "elsewhere"), "elsewhere\n")
	missing := filepath.Join(homeDir, "gone")

	tests := []struct {
		name string
		p    string
		mode os.FileMode
		want string
	}{
		{
			name: "a link into another repository",
			p:    symlink(t, managed, filepath.Join(homeDir, "managed")),
			mode: os.ModeSymlink,
			want: "Warning: skipping " + filepath.Join(homeDir, "managed") +
				" (repository other already manages it; remove it from there first)\n",
		},
		{
			name: "a link to anywhere else",
			p:    symlink(t, elsewhere, filepath.Join(homeDir, "link")),
			mode: os.ModeSymlink,
			want: "Warning: skipping symbolic link " + filepath.Join(homeDir, "link") +
				" -> " + elsewhere + " (add that path instead)\n",
		},
		{
			name: "a link whose target is missing",
			p:    symlink(t, missing, filepath.Join(homeDir, "broken")),
			mode: os.ModeSymlink,
			want: "Warning: skipping symbolic link " + filepath.Join(homeDir, "broken") +
				" -> " + missing + " (add that path instead)\n",
		},
		{
			name: "a link into the repository being added to",
			p:    symlink(t, writeFile(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".inputrc"), "inputrc\n"), filepath.Join(homeDir, ".inputrc")),
			mode: os.ModeSymlink,
			want: "",
		},
		{
			name: "an irregular file",
			p:    filepath.Join(homeDir, "socket"),
			mode: os.ModeSocket,
			want: "Warning: skipping socket " + filepath.Join(homeDir, "socket") + " (git cannot store it)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The paths here are already named as the caller typed them, so
			// the two roots are the same and nothing is rewritten.
			got := testout.Capture(t, func() { reportSkipped(repoPath, homeDir, homeDir)(tt.p, tt.mode) })

			if got != tt.want {
				t.Errorf("reportSkipped() printed %q, want %q", got, tt.want)
			}
		})
	}
}

// Adding a directory again meets the links the first add left behind, which
// this repository put there
func TestAddPathsIsQuietWhenItMeetsItsOwnLinks(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	conf := filepath.Join(homeDir, ".conf")
	writeFile(t, filepath.Join(conf, "one"), "one\n")
	if err := AddPaths(repoPath, false, []string{conf}); err != nil {
		t.Fatal(err)
	}
	// `gog add` links what it copied, which is the state a second add meets
	if err := os.Remove(filepath.Join(conf, "one")); err != nil {
		t.Fatal(err)
	}
	symlink(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".conf", "one"), filepath.Join(conf, "one"))
	writeFile(t, filepath.Join(conf, "two"), "two\n")

	out := testout.Capture(t, func() {
		if err := AddPaths(repoPath, false, []string{conf}); err != nil {
			t.Fatalf("AddPaths() = %v", err)
		}
	})

	if out != "" {
		t.Errorf("AddPaths() printed %q, want nothing", out)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ContentDirName, "$HOME", ".conf", "two")); err != nil {
		t.Errorf("the repository does not hold the path that was added: %v", err)
	}
}

// The warning reaches the copy, and the rest of the directory is still added
func TestAddPathsReportsAPathAnotherRepositoryManages(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	managed := writeFile(t, filepath.Join(BaseDir, "other", ContentDirName, "$HOME", ".conf", "one"), "one\n")
	conf := filepath.Join(homeDir, ".conf")
	writeFile(t, filepath.Join(conf, "two"), "two\n")
	symlink(t, managed, filepath.Join(conf, "one"))

	out := testout.Capture(t, func() {
		if err := AddPaths(repoPath, false, []string{conf}); err != nil {
			t.Fatalf("AddPaths() = %v", err)
		}
	})

	want := "Warning: skipping " + filepath.Join(conf, "one") +
		" (repository other already manages it; remove it from there first)\n"
	if !strings.Contains(out, want) {
		t.Errorf("AddPaths() printed %q, want %q", out, want)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ContentDirName, "$HOME", ".conf", "two")); err != nil {
		t.Errorf("the repository does not hold the rest of the directory: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(repoPath, ContentDirName, "$HOME", ".conf", "one")); !os.IsNotExist(err) {
		t.Error("the repository took a path that another repository manages")
	}
}

// `git rm` deletes what it tracked; a path that was only ever copied in has to
// be cleared separately, or the repository keeps a copy of it
func TestRemovePathsUntracksAndLeavesNothingBehind(t *testing.T) {
	tests := []struct {
		name      string
		committed bool
	}{
		{name: "a path git tracks", committed: true},
		{name: "one that was only copied in"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, homeDir := newSandbox(t)
			target := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")
			if err := AddPaths(repoPath, false, []string{target}); err != nil {
				t.Fatal(err)
			}
			if tt.committed {
				gittest.Run(t, repoPath, "add", "-A")
				gittest.Run(t, repoPath, "commit", "-q", "-m", "init")
			}

			if err := RemovePaths(repoPath, []string{target}); err != nil {
				t.Fatalf("RemovePaths() = %v", err)
			}

			if _, err := os.Lstat(filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc")); !os.IsNotExist(err) {
				t.Errorf("the repository still holds the path (%v)", err)
			}
			if tracked := gittest.Run(t, repoPath, "ls-files"); strings.Contains(tracked, ".bashrc") {
				t.Errorf("the index still holds %q", tracked)
			}
		})
	}
}

// Removing a path a repository never held is reported rather than passed over:
// it looks the same as one that was just given back
func TestRemovePathsReportsAPathItNeverHeld(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	target := filepath.Join(homeDir, ".never")

	out := testout.Capture(t, func() {
		if err := RemovePaths(repoPath, []string{target}); err != nil {
			t.Errorf("RemovePaths() = %v, want success", err)
		}
	})

	want := "Skipped: " + target + " (not tracked by dots)"

	if !strings.Contains(out, want) {
		t.Errorf("RemovePaths() printed %q, want %q", out, want)
	}
}

func TestUnsavedWorkCountsWhatDeletionWouldDestroy(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	target := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")
	if err := AddPaths(repoPath, false, []string{target}); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, repoPath, "add", "-A")
	gittest.Run(t, repoPath, "commit", "-q", "-m", "init")

	// A repository with no remote holds its whole history nowhere else
	unsaved, err := UnsavedWork(repoPath)
	if err != nil {
		t.Fatalf("UnsavedWork() = %v", err)
	}
	if len(unsaved) != 1 || !strings.Contains(unsaved[0], "1 commit that no remote has") {
		t.Errorf("UnsavedWork() = %v, want the one commit", unsaved)
	}

	// Once a remote holds it, only what is not committed remains
	remote := filepath.Join(t.TempDir(), "origin.git")
	gittest.Run(t, repoPath, "init", "-q", "--bare", remote)
	gittest.Run(t, repoPath, "remote", "add", "origin", remote)
	gittest.Run(t, repoPath, "push", "-q", "-u", "origin", "HEAD")
	writeFile(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".vimrc"), "vimrc\n")

	unsaved, err = UnsavedWork(repoPath)
	if err != nil {
		t.Fatalf("UnsavedWork() = %v", err)
	}
	if len(unsaved) != 1 || !strings.Contains(unsaved[0], "1 uncommitted change") {
		t.Errorf("UnsavedWork() = %v, want the one uncommitted change", unsaved)
	}
}

// A name that selects a repository for deletion is given in full, so that a
// short one cannot match something the user did not mean
func TestRemovalPathRefusesAPrefix(t *testing.T) {
	_, _ = newSandbox(t)

	if _, err := RemovalPath("dot"); err == nil {
		t.Error("RemovalPath(\"dot\") resolved a prefix")
	}
	if _, err := RemovalPath(""); err == nil {
		t.Error("RemovalPath(\"\") resolved the default repository")
	}
	if _, err := RemovalPath("dots"); err != nil {
		t.Errorf("RemovalPath(\"dots\") = %v, want the repository", err)
	}
}

// The name comes from the environment rather than the command line, so the
// failure has to say where it came from
func TestGetDefaultNamesTheEnvironmentInItsFailure(t *testing.T) {
	_, _ = newSandbox(t)
	t.Setenv("GOG_DEFAULT_REPOSITORY_NAME", "missing")

	_, err := GetDefault()

	if err == nil || !strings.Contains(err.Error(), "GOG_DEFAULT_REPOSITORY_NAME") {
		t.Errorf("GetDefault() = %v, want a failure naming the variable", err)
	}
}

func TestDescribePathError(t *testing.T) {
	other := errors.New("something else")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "missing", err: &fs.PathError{Op: "lstat", Path: "/x", Err: fs.ErrNotExist}, want: `path "/given" does not exist`},
		{name: "unreadable", err: &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}, want: `cannot read "/given": permission denied`},
		{name: "anything else is passed through", err: other, want: other.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := describePathError("/given", tt.err); got.Error() != tt.want {
				t.Errorf("describePathError() = %q, want %q", got, tt.want)
			}
		})
	}
}

// git explains a wait status itself, so gog names only its own step; anything
// else means git never ran and nothing else says why
func TestGitFailureKeepsTheCauseWhenGitNeverRan(t *testing.T) {
	notFound := exec.Command("gog-no-such-program").Run()
	if err := gitFailure(notFound, "failed to clone x"); err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Errorf("gitFailure() = %v, want the cause kept", err)
	}

	waitStatus := exec.Command("false").Run()
	if err := gitFailure(waitStatus, "failed to clone x"); err == nil || err.Error() != "failed to clone x" {
		t.Errorf("gitFailure() = %v, want the step alone", err)
	}
	if err := gitFailure(nil, "failed to clone x"); err != nil {
		t.Errorf("gitFailure(nil) = %v, want nil", err)
	}
}

func TestQuantify(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{{1, "1 commit"}, {2, "2 commits"}, {0, "0 commits"}}
	for _, tt := range tests {
		if got := quantify(tt.n, "commit"); got != tt.want {
			t.Errorf("quantify(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{{"", 0}, {"  \n ", 0}, {"one\n", 1}, {"one\ntwo\n", 2}, {"one\ntwo", 2}}
	for _, tt := range tests {
		if got := countLines(tt.in); got != tt.want {
			t.Errorf("countLines(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// Git records only the executable bit, so a mode that withholds access from
// group or other is widened wherever the repository is applied
func TestWidenedMode(t *testing.T) {
	tests := []struct {
		name        string
		mode        os.FileMode
		wantMode    os.FileMode
		wantWidened bool
	}{
		{name: "a private file", mode: 0600, wantMode: 0644, wantWidened: true},
		{name: "a private executable", mode: 0700, wantMode: 0755, wantWidened: true},
		{name: "an ordinary file", mode: 0644, wantMode: 0644},
		{name: "an ordinary executable", mode: 0755, wantMode: 0755},
		{name: "a more permissive file is only tightened", mode: 0666, wantMode: 0644},
		{name: "a private directory", mode: os.ModeDir | 0700, wantMode: 0755, wantWidened: true},
		{name: "an ordinary directory", mode: os.ModeDir | 0755, wantMode: 0755},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotWidened := widenedMode(tt.mode)
			if gotMode != tt.wantMode || gotWidened != tt.wantWidened {
				t.Errorf("widenedMode(%04o) = (%04o, %v), want (%04o, %v)",
					tt.mode, gotMode, gotWidened, tt.wantMode, tt.wantWidened)
			}
		})
	}
}

// A copy that fails part-way leaves nothing behind, so that the repository does
// not hold files that were never linked or staged. A path it already held is
// reported instead, because discarding that would throw away whatever the copy
// had overwritten.
func TestAddPathsUndoesAFailedCopy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory whose mode forbids it")
	}
	repoPath, homeDir := newSandbox(t)
	conf := filepath.Join(homeDir, ".conf")
	writeFile(t, filepath.Join(conf, "a", "file"), "file\n")
	// Sorted last, so the copy fails only once it has written something
	unreadable := filepath.Join(conf, "z")
	if err := os.Mkdir(unreadable, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0755) })
	intPath := filepath.Join(repoPath, ContentDirName, "$HOME", ".conf")

	if err := AddPaths(repoPath, false, []string{conf}); err == nil {
		t.Fatal("AddPaths() reported success for a copy that failed")
	}
	if _, err := os.Lstat(intPath); !os.IsNotExist(err) {
		t.Errorf("the repository holds a partial copy (%v)", err)
	}

	held := writeFile(t, filepath.Join(intPath, "held"), "held\n")

	err := AddPaths(repoPath, false, []string{conf})

	if err == nil || !strings.Contains(err.Error(), "dots still holds a partial copy of "+conf) {
		t.Fatalf("AddPaths() = %v, want the partial copy reported", err)
	}
	if contents, readErr := os.ReadFile(held); readErr != nil || string(contents) != "held\n" {
		t.Errorf("the repository holds %q (%v), want what it held", contents, readErr)
	}
}

// A mode git cannot record is reported, so that a private file is not silently
// widened on the next machine that applies the repository
func TestAddPathsWarnsAboutAModeGitCannotRecord(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	private := filepath.Join(homeDir, ".netrc")
	if err := os.WriteFile(private, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Added alongside it, and not warned about
	ordinary := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")

	out := testout.Capture(t, func() {
		if err := AddPaths(repoPath, false, []string{private, ordinary}); err != nil {
			t.Fatalf("AddPaths() = %v", err)
		}
	})

	want := "Warning: " + private + " has mode 0600, which git does not record;" +
		" it will be applied as 0644 on another machine\n"
	if out != want {
		t.Errorf("AddPaths() printed %q, want %q", out, want)
	}
}

// A temporary directory on macOS reaches gog's data directory through a
// symbolic link. Adding a path the repository already holds once compared the
// resolved file against the unresolved one, missed, and copied it over itself.
func TestAddPathsThroughASymlinkedDataDirectoryKeepsWhatItHolds(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	held := writeFile(t, filepath.Join(repoPath, ContentDirName, "$HOME", ".bashrc"), "bashrc\n")
	target := symlink(t, held, filepath.Join(homeDir, ".bashrc"))

	linked := filepath.Join(t.TempDir(), "data-link")
	if err := os.Symlink(BaseDir, linked); err != nil {
		t.Fatal(err)
	}
	// Restored by the sandbox's own cleanup.
	BaseDir = linked

	if err := AddPaths(filepath.Join(linked, filepath.Base(repoPath)), false, []string{target}); err != nil {
		t.Fatalf("AddPaths() = %v", err)
	}

	contents, err := os.ReadFile(held)
	if err != nil || string(contents) != "bashrc\n" {
		t.Errorf("the repository holds %q (%v), want what it held", contents, err)
	}
}
