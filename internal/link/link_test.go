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

// newSandbox creates a repository and a home directory to link into, and points
// the repository package at both for the duration of the test
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
	gitInit(t, repoPath)

	// Whatever the developer has set would otherwise decide what is linked
	t.Setenv("GOG_IGNORE_FILES_REGEX", "")

	originalHome := repository.SetHomeDirForTest(homeDir)
	originalBase := repository.BaseDir
	repository.BaseDir = baseDir
	t.Cleanup(func() {
		repository.SetHomeDirForTest(originalHome)
		repository.BaseDir = originalBase
	})
	return repoPath, homeDir
}

// gitInit initializes a repository that can commit without a global git config
func gitInit(t *testing.T, repoPath string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestFileLinksAndStagesOneFile(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")

	if err := File(repoPath, intPath); err != nil {
		t.Fatalf("File() = %v", err)
	}

	assertLink(t, filepath.Join(homeDir, ".bashrc"), intPath)
	if got := staged(t, repoPath); !strings.Contains(got, "$HOME/.bashrc") {
		t.Errorf("index holds %q, want the linked path", got)
	}
}

func TestFileSkipsAnIgnoredFile(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	t.Setenv("GOG_IGNORE_FILES_REGEX", `\.swp$`)
	intPath := write(t, repoPath, "$HOME/.bashrc.swp", "swp\n")

	if err := File(repoPath, intPath); err != nil {
		t.Fatalf("File() = %v", err)
	}

	if _, err := os.Lstat(filepath.Join(homeDir, ".bashrc.swp")); !os.IsNotExist(err) {
		t.Errorf("a path the pattern matches was linked (%v)", err)
	}
}

// A directory is left alone rather than removed, and the caller still has to
// learn that the run was incomplete
func TestFileRefusesADirectoryInTheWay(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.config", "config\n")
	extPath := filepath.Join(homeDir, ".config")
	if err := os.MkdirAll(extPath, 0755); err != nil {
		t.Fatal(err)
	}

	err := File(repoPath, intPath)

	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("File() = %v, want ErrIncomplete", err)
	}
	if info, statErr := os.Lstat(extPath); statErr != nil || !info.IsDir() {
		t.Errorf("%s is no longer a directory (%v)", extPath, statErr)
	}
}

// Link is given paths as they are named outside the repository, and hands each
// to Dir or File by what the repository holds at it
func TestLinkDispatchesOnWhatTheRepositoryHolds(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	fileIntPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	dirIntPath := write(t, repoPath, "$HOME/.config/app/conf", "conf\n")
	unheld := filepath.Join(homeDir, ".vimrc")

	err := Link(repoPath, []string{
		filepath.Join(homeDir, ".bashrc"),
		filepath.Join(homeDir, ".config"),
		unheld,
	})

	if err != nil {
		t.Fatalf("Link() = %v", err)
	}
	assertLink(t, filepath.Join(homeDir, ".bashrc"), fileIntPath)
	assertLink(t, filepath.Join(homeDir, ".config/app/conf"), dirIntPath)
	// A path the repository does not hold is passed over rather than failing the
	// run: `gog add` links what it just copied, and nothing else is its business
	if _, statErr := os.Lstat(unheld); !os.IsNotExist(statErr) {
		t.Errorf("a path the repository does not hold was acted on (%v)", statErr)
	}
}
