package repository

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	for _, p := range []string{repoPath, homeDir} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, repoPath, "init", "-q")
	runGit(t, repoPath, "config", "user.email", "test@example.com")
	runGit(t, repoPath, "config", "user.name", "Test User")

	originalBase := BaseDir
	BaseDir = baseDir
	originalHome := SetHomeDirForTest(homeDir)
	t.Cleanup(func() {
		BaseDir = originalBase
		SetHomeDirForTest(originalHome)
	})
	return repoPath, homeDir
}

func runGit(t *testing.T, repoPath string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
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

// A path in the home directory is stored under a literal $HOME component, which
// is what makes a repository portable between machines
func TestAddPathsStoresHomePathsPortably(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	target := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")

	if err := AddPaths(repoPath, []string{target}); err != nil {
		t.Fatalf("AddPaths() = %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(repoPath, "$HOME", ".bashrc"))
	if err != nil || string(contents) != "bashrc\n" {
		t.Errorf("the repository holds %q (%v), want the file's contents", contents, err)
	}
}

func TestAddPathsCopiesADirectoryTree(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	writeFile(t, filepath.Join(homeDir, ".config/app/conf"), "conf\n")
	writeFile(t, filepath.Join(homeDir, ".config/app/nested/deep.conf"), "deep\n")

	if err := AddPaths(repoPath, []string{filepath.Join(homeDir, ".config")}); err != nil {
		t.Fatalf("AddPaths() = %v", err)
	}

	for _, rel := range []string{"$HOME/.config/app/conf", "$HOME/.config/app/nested/deep.conf"} {
		if _, err := os.Stat(filepath.Join(repoPath, rel)); err != nil {
			t.Errorf("the repository does not hold %s: %v", rel, err)
		}
	}
}

// The whole batch is checked before anything is copied, so that one unusable
// path does not leave the repository holding files that were never linked
func TestAddPathsChecksEveryPathBeforeCopyingAny(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	good := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")
	missing := filepath.Join(homeDir, ".gone")

	err := AddPaths(repoPath, []string{good, missing})

	if err == nil || !strings.Contains(err.Error(), "path does not exist") {
		t.Fatalf("AddPaths() = %v, want a failure naming the missing path", err)
	}
	if _, statErr := os.Stat(filepath.Join(repoPath, "$HOME", ".bashrc")); !os.IsNotExist(statErr) {
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
			name: "a named pipe has nothing git can store",
			prepare: func(t *testing.T, homeDir string) string {
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
				return filepath.Join(homeDir, ".gone")
			},
			want: "path does not exist",
		},
		{
			name: "gog's own data directory",
			prepare: func(t *testing.T, _ string) string {
				return filepath.Join(BaseDir, "dots", "$HOME", ".bashrc")
			},
			want: "gog's own directory cannot be managed",
		},
		{
			name: "a backup left by an older version",
			prepare: func(t *testing.T, homeDir string) string {
				return writeFile(t, filepath.Join(homeDir, ".bashrc.gog"), "old\n")
			},
			want: ".gog backup files cannot be managed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, homeDir := newSandbox(t)
			target := tt.prepare(t, homeDir)

			err := AddPaths(repoPath, []string{target})

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("AddPaths() = %v, want a failure containing %q", err, tt.want)
			}
		})
	}
}

func TestRemovePathsUntracksAndLeavesNothingBehind(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	target := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")
	if err := AddPaths(repoPath, []string{target}); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoPath, "add", "-A")
	runGit(t, repoPath, "commit", "-q", "-m", "init")

	if err := RemovePaths(repoPath, []string{target}); err != nil {
		t.Fatalf("RemovePaths() = %v", err)
	}

	if _, err := os.Lstat(filepath.Join(repoPath, "$HOME", ".bashrc")); !os.IsNotExist(err) {
		t.Errorf("the repository still holds the path (%v)", err)
	}
	if tracked := runGit(t, repoPath, "ls-files"); strings.Contains(tracked, ".bashrc") {
		t.Errorf("the index still holds %q", tracked)
	}
}

// Removing a path a repository never held is reported rather than passed over,
// because it looks exactly like one it just gave back
func TestRemovePathsReportsAPathItNeverHeld(t *testing.T) {
	repoPath, homeDir := newSandbox(t)

	if err := RemovePaths(repoPath, []string{filepath.Join(homeDir, ".never")}); err != nil {
		t.Errorf("RemovePaths() = %v, want success", err)
	}
}

func TestUnsavedWorkCountsWhatDeletionWouldDestroy(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	target := writeFile(t, filepath.Join(homeDir, ".bashrc"), "bashrc\n")
	if err := AddPaths(repoPath, []string{target}); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoPath, "add", "-A")
	runGit(t, repoPath, "commit", "-q", "-m", "init")

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
	runGit(t, repoPath, "init", "-q", "--bare", remote)
	runGit(t, repoPath, "remote", "add", "origin", remote)
	runGit(t, repoPath, "push", "-q", "-u", "origin", "HEAD")
	writeFile(t, filepath.Join(repoPath, "$HOME", ".vimrc"), "vimrc\n")

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
		{name: "missing", err: &fs.PathError{Op: "lstat", Path: "/x", Err: fs.ErrNotExist}, want: "path does not exist: /given"},
		{name: "unreadable", err: &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}, want: "cannot read /given: permission denied"},
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
