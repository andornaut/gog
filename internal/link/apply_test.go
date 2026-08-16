package link

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andornaut/gog/internal/gittest"
	"github.com/andornaut/gog/internal/repository"
	"github.com/andornaut/gog/internal/testout"
)

// write creates a file in the repository at a path given relative to the
// directory whose tree is linked, and returns the path it holds
func write(t *testing.T, repoPath, rel, contents string) string {
	t.Helper()
	p := filepath.Join(repoPath, repository.ContentDirName, rel)
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
	return gittest.Run(t, repoPath, "ls-files")
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

	if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	assertLink(t, filepath.Join(homeDir, ".config/app/conf"), intPath)
	// The directories on the way are created rather than linked, so that a
	// repository that holds part of a directory does not claim the whole of it.
	// Lstat reports a link as neither a directory nor a regular file.
	app := filepath.Join(homeDir, ".config/app")
	if info, err := os.Lstat(app); err != nil || !info.IsDir() {
		t.Errorf("%s is not a real directory (%v)", app, err)
	}
	if got := staged(t, repoPath); !strings.Contains(got, "$HOME/.config/app/conf") {
		t.Errorf("index holds %q, want the linked path", got)
	}
}

// A repository with no content directory has nothing to link, and its own files
// sit beside that directory rather than in it
func TestDirOnARepositoryWithNoContentDirectory(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	for _, name := range []string{"README.md", ".gitignore"} {
		if err := os.WriteFile(filepath.Join(repoPath, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	entries, err := os.ReadDir(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the home directory holds %d entries, want none", len(entries))
	}
}

// The pattern is matched against the path under the content directory, so an
// anchored one names what it appears to rather than needing a root/ prefix
func TestDirHonoursTheIgnorePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{name: "a directory prefix", pattern: `\.cache/`},
		{name: "an anchored path", pattern: `^\$HOME/\.cache/state$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, homeDir := newSandbox(t)
			write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
			write(t, repoPath, "$HOME/.cache/state", "state\n")
			t.Setenv("GOG_IGNORE_FILES_REGEX", tt.pattern)

			if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
				t.Fatalf("Dir() = %v", err)
			}

			if _, err := os.Lstat(filepath.Join(homeDir, ".bashrc")); err != nil {
				t.Errorf("a path the pattern does not match was not linked: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(homeDir, ".cache/state")); !os.IsNotExist(err) {
				t.Errorf("a path the pattern matches was linked (%v)", err)
			}
		})
	}
}

// An unset pattern ignores nothing, whatever pattern was read before it
func TestDirWithTheIgnorePatternUnset(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	extPath := filepath.Join(homeDir, ".bashrc")
	t.Setenv("GOG_IGNORE_FILES_REGEX", `\.bashrc$`)
	if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
		t.Fatalf("Dir() = %v", err)
	}
	if _, err := os.Lstat(extPath); !os.IsNotExist(err) {
		t.Fatalf("the pattern was never read (%v)", err)
	}

	t.Setenv("GOG_IGNORE_FILES_REGEX", "")

	if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
		t.Fatalf("Dir() = %v", err)
	}
	if _, err := os.Lstat(extPath); err != nil {
		t.Errorf("a pattern that is no longer set still decides what is linked: %v", err)
	}
}

// A second run leaves the link alone rather than recreating it, which would
// report every path of an already-applied repository as newly linked
func TestDirIsIdempotent(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "one\n")
	extPath := filepath.Join(homeDir, ".bashrc")

	if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
		t.Fatalf("Dir() = %v", err)
	}
	out := testout.Capture(t, func() {
		if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
			t.Errorf("Dir() on the second run = %v", err)
		}
	})

	assertLink(t, extPath, intPath)
	if out != "" {
		t.Errorf("the second run printed %q, want nothing", out)
	}
}

// Every entry point reads the pattern for itself, so a pattern that cannot be
// compiled fails the command that reads it rather than every command
func TestEntryPointsRejectAnUncompilablePattern(t *testing.T) {
	tests := []struct {
		name string
		call func(repoPath, intPath, extPath string) error
	}{
		{name: "Dir", call: func(repoPath, _, _ string) error { return Dir(repoPath, repository.ContentPath(repoPath)) }},
		{name: "File", call: func(repoPath, intPath, _ string) error { return File(repoPath, intPath) }},
		{name: "Link", call: func(repoPath, _, extPath string) error { return Link(repoPath, []string{extPath}) }},
		{name: "List", call: func(repoPath, _, _ string) error { _, err := List(repoPath); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, homeDir := newSandbox(t)
			intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
			extPath := filepath.Join(homeDir, ".bashrc")
			t.Setenv("GOG_IGNORE_FILES_REGEX", "[")

			err := tt.call(repoPath, intPath, extPath)

			if err == nil || !strings.Contains(err.Error(), "GOG_IGNORE_FILES_REGEX") {
				t.Errorf("%s() = %v, want a failure naming the variable", tt.name, err)
			}
			if _, statErr := os.Lstat(extPath); !os.IsNotExist(statErr) {
				t.Error("a path was linked although the pattern could not be compiled")
			}
		})
	}
}

// One path in the way must not stop the rest, and the caller still has to learn
// that the run was incomplete
func TestDirReportsAConflictAndCarriesOn(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	mine := filepath.Join(homeDir, ".bashrc")
	// The same length as what is in the way, so the contents decide
	write(t, repoPath, "$HOME/.bashrc", "them\n")
	otherIntPath := write(t, repoPath, "$HOME/.vimrc", "vimrc\n")
	if err := os.WriteFile(mine, []byte("mine\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Dir(repoPath, repository.ContentPath(repoPath))

	if !errors.Is(err, ErrIncomplete) {
		t.Errorf("Dir() = %v, want ErrIncomplete", err)
	}
	contents, readErr := os.ReadFile(mine)
	if readErr != nil || string(contents) != "mine\n" {
		t.Errorf("%s holds %q (%v), want it left alone", mine, contents, readErr)
	}
	assertLink(t, filepath.Join(homeDir, ".vimrc"), otherIntPath)
}

// Creating a directory over a symbolic link writes through it into whatever it
// points at, so a link of the user's is reported and its tree passed over
func TestDirRefusesToWriteThroughASymlinkedDirectory(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.config/app/conf", "conf\n")
	elsewhere := filepath.Join(homeDir, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(homeDir, ".config")); err != nil {
		t.Fatal(err)
	}

	err := Dir(repoPath, repository.ContentPath(repoPath))

	if !errors.Is(err, ErrIncomplete) {
		t.Errorf("Dir() = %v, want ErrIncomplete", err)
	}
	if entries, readErr := os.ReadDir(elsewhere); readErr != nil || len(entries) != 0 {
		t.Errorf("%s holds %d entries (%v), want nothing written through the link", elsewhere, len(entries), readErr)
	}
}

// A link into gog's data directory holds nothing of the user's, so the
// directory replaces it rather than being written through it
func TestDirReplacesASymlinkedDirectoryOfItsOwn(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.config/app/conf", "conf\n")
	other := filepath.Join(repository.BaseDir, "other", repository.ContentDirName, "$HOME", ".config")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(homeDir, ".config")); err != nil {
		t.Fatal(err)
	}

	if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	if info, err := os.Lstat(filepath.Join(homeDir, ".config")); err != nil || !info.IsDir() {
		t.Errorf("%s is not a real directory (%v)", filepath.Join(homeDir, ".config"), err)
	}
	assertLink(t, filepath.Join(homeDir, ".config/app/conf"), intPath)
}

// A path that cannot be linked at all fails the command, naming what could not
// be done; only a conflict is reported and passed over. What was linked before
// the failure is still staged, so that it is not left untracked.
func TestDirFailsWhenTheLinkCannotBeMade(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to a directory whose mode forbids it")
	}
	repoPath, homeDir := newSandbox(t)
	// Walked first, so it is linked before the failure below
	linked := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	write(t, repoPath, "$HOME/locked/conf", "conf\n")
	locked := filepath.Join(homeDir, "locked")
	if err := os.Mkdir(locked, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0755) })

	err := Dir(repoPath, repository.ContentPath(repoPath))

	if err == nil || !strings.Contains(err.Error(), "failed to create symlink") {
		t.Errorf("Dir() = %v, want a failure naming what could not be done", err)
	}
	if _, statErr := os.Lstat(filepath.Join(locked, "conf")); !os.IsNotExist(statErr) {
		t.Errorf("the path was linked although the directory forbids it (%v)", statErr)
	}
	assertLink(t, filepath.Join(homeDir, ".bashrc"), linked)
	if got := staged(t, repoPath); !strings.Contains(got, "$HOME/.bashrc") {
		t.Errorf("index holds %q, want the path linked before the failure", got)
	}
}

// The cases where nothing of the user's is lost, so applying replaces what is
// in the way without asking
func TestDirReplacesWhatHoldsNothingOfTheUsers(t *testing.T) {
	tests := []struct {
		name     string
		inTheWay func(t *testing.T, homeDir, otherRepo string)
	}{
		{
			name: "a broken symbolic link",
			inTheWay: func(t *testing.T, homeDir, _ string) {
				t.Helper()
				if err := os.Symlink(filepath.Join(homeDir, "gone"), filepath.Join(homeDir, ".bashrc")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a link into gog's data directory",
			inTheWay: func(t *testing.T, homeDir, otherRepo string) {
				t.Helper()
				other := filepath.Join(otherRepo, repository.ContentDirName, "$HOME", ".bashrc")
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
			// What `gog add` leaves behind, having copied the file in first
			name: "a copy of what the repository holds",
			inTheWay: func(t *testing.T, homeDir, _ string) {
				t.Helper()
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
			tt.inTheWay(t, homeDir, otherRepo)

			if err := Dir(repoPath, repository.ContentPath(repoPath)); err != nil {
				t.Fatalf("Dir() = %v", err)
			}
			assertLink(t, filepath.Join(homeDir, ".bashrc"), intPath)
		})
	}
}
