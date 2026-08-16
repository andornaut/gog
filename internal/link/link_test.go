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
	gittest.Init(t, repoPath)
	gittest.Isolate(t, homeDir)

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
// $HOME component escaped so that the path can be pasted into a shell. Taken
// from linkFile because File also stages, and git writes to the same stream.
func TestLinkFilePrintsWhatItLinked(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")

	out := testout.Capture(t, func() {
		if _, err := linkFile(repoPath, intPath); err != nil {
			t.Errorf("linkFile() = %v", err)
		}
	})

	want := "Linked: " + filepath.Join(homeDir, ".bashrc") + " -> " +
		filepath.Join(repoPath, repository.ContentDirName, `\$HOME`, ".bashrc") + "\n"
	if out != want {
		t.Errorf("linkFile() printed %q, want %q", out, want)
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

	out := testout.Capture(t, func() {
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

	out := testout.Capture(t, func() {
		if err := File(repoPath, intPath); !errors.Is(err, ErrIncomplete) {
			t.Errorf("File() = %v, want ErrIncomplete", err)
		}
	})

	if info, statErr := os.Lstat(extPath); statErr != nil || !info.IsDir() {
		t.Errorf("%s is no longer a directory (%v)", extPath, statErr)
	}
	if !strings.Contains(out, extPath+" exists and is a directory") {
		t.Errorf("File() printed %q, want the directory named", out)
	}
}

// Two files of the same length are compared byte for byte. A wrong answer here
// deletes a file of the user's that the repository does not hold.
func TestSameContents(t *testing.T) {
	// Longer than the buffer a comparison reads into, so the loop runs twice
	large := strings.Repeat("ab", 64*1024)

	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "identical", a: "same\n", b: "same\n", want: true},
		{name: "the same length, differing", a: "mine\n", b: "them\n"},
		{name: "differing lengths", a: "short\n", b: "longer\n"},
		{name: "both empty", a: "", b: "", want: true},
		{name: "past one read, identical", a: large, b: large, want: true},
		{name: "past one read, differing at the end", a: large, b: large[:len(large)-1] + "z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
			for p, contents := range map[string]string{a: tt.a, b: tt.b} {
				if err := os.WriteFile(p, []byte(contents), 0644); err != nil {
					t.Fatal(err)
				}
			}

			if got := sameContents(a, b); got != tt.want {
				t.Errorf("sameContents(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// Replacing a symbolic link discards the link itself, which no copy of the
// contents preserves
func TestSameContentsExcludesASymbolicLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("same\n"), 0644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	if sameContents(linkPath, target) {
		t.Error("sameContents() called a symbolic link the same as what it points at")
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
