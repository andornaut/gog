package link

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andornaut/gog/internal/repository"
)

// captureStderr returns what f writes to standard error. What gog did goes
// there rather than through a writer the caller supplies, so a test that means
// to check one has to take it from the process.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = original }()

	done := make(chan string)
	go func() {
		var out strings.Builder
		_, _ = io.Copy(&out, reader)
		done <- out.String()
	}()

	f()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return <-done
}

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

// The result line names both ends of the link, with the repository's literal
// $HOME component escaped so that the path can be pasted into a shell
func TestFilePrintsWhatItLinked(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")

	out := captureStderr(t, func() {
		if err := File(repoPath, intPath); err != nil {
			t.Fatalf("File() = %v", err)
		}
	})

	want := "Linked: " + filepath.Join(homeDir, ".bashrc") + " -> " +
		filepath.Join(repoPath, `\$HOME`, ".bashrc") + "\n"
	if out != want {
		t.Errorf("File() printed %q, want %q", out, want)
	}
}

// A path git will not stage is reported, and the link is still made
func TestFileReportsAPathGitWillNotStage(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	// The lock an interrupted or concurrent git leaves behind
	if err := os.WriteFile(filepath.Join(repoPath, ".git", "index.lock"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	out := captureStderr(t, func() {
		if err := File(repoPath, intPath); !errors.Is(err, ErrIncomplete) {
			t.Errorf("File() = %v, want ErrIncomplete", err)
		}
	})

	assertLink(t, filepath.Join(homeDir, ".bashrc"), intPath)
	if !strings.Contains(out, "Error: failed to add "+intPath+" to git") {
		t.Errorf("File() printed %q, want the path git would not stage", out)
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

// Link takes paths as they are named outside the repository, and hands each to
// Dir or File by what the repository holds at it
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
	// A path the repository does not hold is passed over rather than failing
	// the run
	if _, statErr := os.Lstat(unheld); !os.IsNotExist(statErr) {
		t.Errorf("a path the repository does not hold was acted on (%v)", statErr)
	}
}
