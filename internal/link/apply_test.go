package link

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andornaut/gog/internal/repository"
)

// write creates a file in the repository at a path given relative to the
// repository root, and returns the path it holds
func write(t *testing.T, repoPath, rel, contents string) string {
	t.Helper()
	p := filepath.Join(repoPath, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

// staged returns the paths that the repository's index holds
func staged(t *testing.T, repoPath string) string {
	t.Helper()
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func assertLink(t *testing.T, extPath, intPath string) {
	t.Helper()
	target, err := os.Readlink(extPath)
	if err != nil {
		t.Fatalf("%s is not a symbolic link: %v", extPath, err)
	}
	if target != intPath {
		t.Errorf("%s -> %s, want %s", extPath, target, intPath)
	}
}

func TestDirLinksAndStagesATree(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.config/app/conf", "conf\n")

	if err := Dir(repoPath, repoPath); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	assertLink(t, filepath.Join(homeDir, ".config/app/conf"), intPath)
	// The directories on the way are created rather than linked, so that a
	// repository that holds part of a directory does not claim the whole of it
	info, err := os.Lstat(filepath.Join(homeDir, ".config/app"))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("%s is not a real directory (%v, %v)", filepath.Join(homeDir, ".config/app"), info, err)
	}
	if got := staged(t, repoPath); !strings.Contains(got, "$HOME/.config/app/conf") {
		t.Errorf("index holds %q, want the linked path", got)
	}
}

func TestDirIsIdempotent(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "one\n")

	for i := range 2 {
		if err := Dir(repoPath, repoPath); err != nil {
			t.Fatalf("Dir() on run %d = %v", i+1, err)
		}
	}
	assertLink(t, filepath.Join(homeDir, ".bashrc"), intPath)
}

func TestDirSkipsWhatIsNeverLinked(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	for _, name := range []string{".gitignore", "LICENSE", "README.md"} {
		write(t, repoPath, name, name)
	}
	write(t, repoPath, "$HOME/.bashrc", "bashrc\n")

	if err := Dir(repoPath, repoPath); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	// A repository's own files stay in it, and .git is never walked into
	for _, name := range []string{".gitignore", "LICENSE", "README.md"} {
		if _, err := os.Lstat(filepath.Join("/", name)); err == nil {
			t.Errorf("%s was linked to the filesystem root", name)
		}
	}
	if _, err := os.Lstat(filepath.Join(homeDir, ".bashrc")); err != nil {
		t.Errorf("the rest of the repository was not linked: %v", err)
	}
	if strings.Contains(staged(t, repoPath), ".git/") {
		t.Error("something under .git was staged")
	}
}

func TestDirHonoursTheIgnorePattern(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	write(t, repoPath, "$HOME/.cache/state", "state\n")
	t.Setenv("GOG_IGNORE_FILES_REGEX", `\.cache/`)

	if err := Dir(repoPath, repoPath); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	if _, err := os.Lstat(filepath.Join(homeDir, ".bashrc")); err != nil {
		t.Errorf("a path the pattern does not match was not linked: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(homeDir, ".cache/state")); !os.IsNotExist(err) {
		t.Errorf("a path the pattern matches was linked (%v)", err)
	}
}

// A pattern that cannot be compiled fails the command that reads it. It used to
// exit the process from an init, which failed every command over a setting only
// the linking commands use.
func TestDirRejectsAnUncompilablePattern(t *testing.T) {
	repoPath, _ := newSandbox(t)
	write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	t.Setenv("GOG_IGNORE_FILES_REGEX", "[")

	err := Dir(repoPath, repoPath)
	if err == nil || !strings.Contains(err.Error(), "GOG_IGNORE_FILES_REGEX") {
		t.Errorf("Dir() = %v, want a failure naming the variable", err)
	}
}

// One path in the way must not stop the rest, and the caller still has to learn
// that the run was incomplete
func TestDirReportsAConflictAndCarriesOn(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	mine := filepath.Join(homeDir, ".bashrc")
	write(t, repoPath, "$HOME/.bashrc", "theirs\n")
	otherIntPath := write(t, repoPath, "$HOME/.vimrc", "vimrc\n")
	if err := os.WriteFile(mine, []byte("mine\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Dir(repoPath, repoPath)

	if !errors.Is(err, ErrIncomplete) {
		t.Errorf("Dir() = %v, want ErrIncomplete", err)
	}
	contents, readErr := os.ReadFile(mine)
	if readErr != nil || string(contents) != "mine\n" {
		t.Errorf("%s holds %q (%v), want it left alone", mine, contents, readErr)
	}
	assertLink(t, filepath.Join(homeDir, ".vimrc"), otherIntPath)
}

// The three cases where nothing of the user's is lost, so applying replaces
// what is in the way without asking
func TestDirReplacesWhatHoldsNothingOfTheUsers(t *testing.T) {
	tests := []struct {
		name     string
		inTheWay func(t *testing.T, homeDir, intPath, otherRepo string)
	}{
		{
			name: "a broken symbolic link",
			inTheWay: func(t *testing.T, homeDir, _, _ string) {
				if err := os.Symlink(filepath.Join(homeDir, "gone"), filepath.Join(homeDir, ".bashrc")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a link into gog's data directory",
			inTheWay: func(t *testing.T, homeDir, _, otherRepo string) {
				other := filepath.Join(otherRepo, "$HOME", ".bashrc")
				if err := os.MkdirAll(filepath.Dir(other), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(other, []byte("other\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, filepath.Join(homeDir, ".bashrc")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a copy of what the repository holds, which is what add leaves behind",
			inTheWay: func(t *testing.T, homeDir, _, _ string) {
				if err := os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("same\n"), 0644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, homeDir := newSandbox(t)
			intPath := write(t, repoPath, "$HOME/.bashrc", "same\n")
			otherRepo := filepath.Join(repository.BaseDir, "other")
			tt.inTheWay(t, homeDir, intPath, otherRepo)

			if err := Dir(repoPath, repoPath); err != nil {
				t.Fatalf("Dir() = %v", err)
			}
			assertLink(t, filepath.Join(homeDir, ".bashrc"), intPath)
		})
	}
}

// A file at the repository root belongs to the filesystem root, which is what
// makes a repository able to hold /etc as well as $HOME. Only the conversion is
// checked here: writing to / is not a test's business.
func TestExternalPathOfARepositoryRootFile(t *testing.T) {
	repoPath, _ := newSandbox(t)
	got := repository.ToExternalPath(repoPath, filepath.Join(repoPath, "etc", "hosts"))
	if got != "/etc/hosts" {
		t.Errorf("ToExternalPath() = %q, want /etc/hosts", got)
	}
}
