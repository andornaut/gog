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

// A second run leaves the link as the first one made it, rather than removing
// and recreating it
func TestDirIsIdempotent(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "one\n")
	extPath := filepath.Join(homeDir, ".bashrc")

	if err := Dir(repoPath, repoPath); err != nil {
		t.Fatalf("Dir() = %v", err)
	}
	first, err := os.Lstat(extPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = Dir(repoPath, repoPath); err != nil {
		t.Fatalf("Dir() on the second run = %v", err)
	}

	assertLink(t, extPath, intPath)
	second, err := os.Lstat(extPath)
	if err != nil {
		t.Fatal(err)
	}
	if !second.ModTime().Equal(first.ModTime()) {
		t.Errorf("the link was recreated (%v, then %v)", first.ModTime(), second.ModTime())
	}
}

// The files a repository keeps for itself sit at its root, which is the
// filesystem root outside it, so they are checked here rather than linked
func TestSkipped(t *testing.T) {
	repoPath, _ := newSandbox(t)
	t.Setenv("GOG_IGNORE_FILES_REGEX", `\.swp$`)
	if err := Configure(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		rel  string
		want bool
	}{
		{rel: ".gitignore", want: true},
		{rel: "LICENSE", want: true},
		{rel: "README.md", want: true},
		{rel: "$HOME/.vimrc.swp", want: true},
		{rel: "$HOME/.bashrc", want: false},
		{rel: "$HOME/.config/README.md", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			if got := skipped(repoPath, filepath.Join(repoPath, tt.rel)); got != tt.want {
				t.Errorf("skipped(%q) = %v, want %v", tt.rel, got, tt.want)
			}
		})
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

// Every entry point reads the pattern for itself, so a pattern that cannot be
// compiled fails the command that reads it rather than every command
func TestEntryPointsRejectAnUncompilablePattern(t *testing.T) {
	tests := []struct {
		name string
		call func(repoPath, intPath, extPath string) error
	}{
		{name: "Dir", call: func(repoPath, _, _ string) error { return Dir(repoPath, repoPath) }},
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

// A path that cannot be linked at all fails the command, naming what could not
// be done. Only a conflict is reported and passed over.
func TestDirFailsWhenTheLinkCannotBeMade(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to a directory whose mode forbids it")
	}
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	if err := os.Chmod(homeDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(homeDir, 0755) })

	err := Dir(repoPath, repoPath)

	if err == nil || !strings.Contains(err.Error(), "failed to create symlink") {
		t.Errorf("Dir() = %v, want a failure naming what could not be done", err)
	}
	if _, statErr := os.Lstat(filepath.Join(homeDir, ".bashrc")); !os.IsNotExist(statErr) {
		t.Errorf("the path was linked although the directory forbids it (%v)", statErr)
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
				if err := os.Symlink(filepath.Join(homeDir, "gone"), filepath.Join(homeDir, ".bashrc")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a link into gog's data directory",
			inTheWay: func(t *testing.T, homeDir, otherRepo string) {
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
			inTheWay: func(t *testing.T, homeDir, _ string) {
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

			if err := Dir(repoPath, repoPath); err != nil {
				t.Fatalf("Dir() = %v", err)
			}
			assertLink(t, filepath.Join(homeDir, ".bashrc"), intPath)
		})
	}
}
