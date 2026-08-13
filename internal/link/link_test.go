package link

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/andornaut/gog/internal/paths"
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

// TestFileCreatesSymlink verifies basic symlink creation
func TestFileCreatesSymlink(t *testing.T) {
	repoPath, _ := newSandbox(t)

	// Create a test file in the repo (using $HOME path format)
	intPath := filepath.Join(repoPath, "$HOME", ".bashrc")
	if mkdirErr := os.MkdirAll(filepath.Dir(intPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(intPath, []byte("test content"), 0644); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create symlink
	err := File(repoPath, intPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify symlink was created
	linkDest, err := os.Readlink(extPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	if linkDest != intPath {
		t.Errorf("Symlink points to %q, want %q", linkDest, intPath)
	}
}

// TestFileRefusesExistingFile verifies that a file the repository did not put
// there is left alone and the failure is reported
func TestFileRefusesExistingFile(t *testing.T) {
	repoPath, _ := newSandbox(t)

	// Create a test file in the repo
	intPath := filepath.Join(repoPath, "$HOME", ".bashrc")
	if mkdirErr := os.MkdirAll(filepath.Dir(intPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(intPath, []byte("new content"), 0644); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create existing file at external path
	if mkdirErr := os.MkdirAll(filepath.Dir(extPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create ext dir: %v", mkdirErr)
	}
	existingContent := []byte("existing content")
	if writeErr := os.WriteFile(extPath, existingContent, 0644); writeErr != nil {
		t.Fatalf("Failed to create existing file: %v", writeErr)
	}

	// Link the file (should refuse, because extPath is the user's)
	err := File(repoPath, intPath)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("File() = %v, want %v", err, ErrIncomplete)
	}

	// Verify the existing file is untouched
	content, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatalf("Existing file not readable: %v", err)
	}
	if string(content) != string(existingContent) {
		t.Errorf("Existing file = %q, want %q", content, existingContent)
	}
	if paths.IsSymlink(extPath) {
		t.Error("Existing file should not have been replaced by a symlink")
	}
}

// TestFileHandlesBrokenSymlink verifies broken symlinks are replaced without backup
func TestFileHandlesBrokenSymlink(t *testing.T) {
	repoPath, _ := newSandbox(t)

	// Create a test file in the repo
	intPath := filepath.Join(repoPath, "$HOME", ".bashrc")
	if mkdirErr := os.MkdirAll(filepath.Dir(intPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(intPath, []byte("test content"), 0644); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create broken symlink at external path
	if mkdirErr := os.MkdirAll(filepath.Dir(extPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create ext dir: %v", mkdirErr)
	}
	brokenTarget := filepath.Join(repoPath, "nonexistent")
	if symlinkErr := os.Symlink(brokenTarget, extPath); symlinkErr != nil {
		t.Fatalf("Failed to create broken symlink: %v", symlinkErr)
	}

	// Create symlink (should replace broken symlink without backup)
	err := File(repoPath, intPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify symlink points to correct location
	linkDest, err := os.Readlink(extPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	if linkDest != intPath {
		t.Errorf("Symlink points to %q, want %q", linkDest, intPath)
	}
}

// TestFileSkipsAlreadyLinked verifies no-op when symlink already correct
func TestFileSkipsAlreadyLinked(t *testing.T) {
	repoPath, _ := newSandbox(t)

	// Create a test file in the repo
	intPath := filepath.Join(repoPath, "$HOME", ".bashrc")
	if mkdirErr := os.MkdirAll(filepath.Dir(intPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(intPath, []byte("test content"), 0644); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create correct symlink
	if mkdirErr := os.MkdirAll(filepath.Dir(extPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create ext dir: %v", mkdirErr)
	}
	if symlinkErr := os.Symlink(intPath, extPath); symlinkErr != nil {
		t.Fatalf("Failed to create initial symlink: %v", symlinkErr)
	}

	// Get initial symlink target
	initialTarget, err := os.Readlink(extPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	// Get initial modification time
	initialInfo, err := os.Lstat(extPath)
	if err != nil {
		t.Fatalf("Failed to stat symlink: %v", err)
	}
	initialModTime := initialInfo.ModTime()

	// Call File() again (should be no-op)
	err = File(repoPath, intPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify symlink target hasn't changed
	finalTarget, err := os.Readlink(extPath)
	if err != nil {
		t.Fatalf("Failed to read symlink after: %v", err)
	}

	if finalTarget != initialTarget {
		t.Errorf("Symlink target changed from %q to %q", initialTarget, finalTarget)
	}

	// Verify symlink wasn't recreated (check modification time hasn't changed)
	finalInfo, err := os.Lstat(extPath)
	if err != nil {
		t.Fatalf("Failed to stat symlink after: %v", err)
	}

	if !finalInfo.ModTime().Equal(initialModTime) {
		t.Errorf("Symlink was recreated (modtime changed from %v to %v)", initialModTime, finalInfo.ModTime())
	}
}

// TestFileSkipsIgnoredFiles verifies GOG_IGNORE_FILES_REGEX pattern matching
func TestFileSkipsIgnoredFiles(t *testing.T) {
	repoPath, _ := newSandbox(t)
	// Set through the environment, which is where the pattern comes from: an
	// entry point that reads it for itself is not exercised by assigning it
	t.Setenv("GOG_IGNORE_FILES_REGEX", `\.swp$`)

	// Create a .swp file in the repo (should be ignored)
	intPath := filepath.Join(repoPath, "$HOME", ".bashrc.swp")
	if mkdirErr := os.MkdirAll(filepath.Dir(intPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(intPath, []byte("test content"), 0644); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create parent directory
	if mkdirErr := os.MkdirAll(filepath.Dir(extPath), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create ext dir: %v", mkdirErr)
	}

	// Attempt to create symlink (should be skipped)
	err := File(repoPath, intPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify symlink was NOT created
	if _, err := os.Lstat(extPath); !os.IsNotExist(err) {
		t.Error("Ignored file should not be linked")
	}
}

// TestFileSkipsSpecialFiles verifies .gitignore, LICENSE, README.md are skipped
func TestFileSkipsSpecialFiles(t *testing.T) {
	repoPath, _ := newSandbox(t)

	specialFiles := []string{".gitignore", "LICENSE", "README.md"}

	for _, filename := range specialFiles {
		t.Run(filename, func(t *testing.T) {
			intPath := filepath.Join(repoPath, filename)
			if err := os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			extPath := repository.ToExternalPath(repoPath, intPath)

			// Attempt to create symlink (should be skipped)
			err := File(repoPath, intPath)
			if err != nil {
				t.Fatalf("File() failed: %v", err)
			}

			// Verify symlink was NOT created
			if _, err := os.Lstat(extPath); !os.IsNotExist(err) {
				t.Errorf("%s should not be linked", filename)
			}
		})
	}
}

// TestFileSkipsExistingDirectory verifies that a directory in the way is left
// alone and reported, rather than removed or silently ignored
func TestFileSkipsExistingDirectory(t *testing.T) {
	repoPath, _ := newSandbox(t)

	// Create a test file in the repo
	intPath := filepath.Join(repoPath, "$HOME", ".config")
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create directory at external path (conflict)
	if err := os.MkdirAll(extPath, 0755); err != nil {
		t.Fatalf("Failed to create conflicting directory: %v", err)
	}

	// Attempt to create symlink. The conflict is reported and the run
	// continues, but the caller must still learn that it was incomplete.
	err := File(repoPath, intPath)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("File() should report an incomplete run for a directory conflict, got: %v", err)
	}

	// Verify directory still exists (unchanged)
	info, err := os.Stat(extPath)
	if err != nil {
		t.Fatalf("Directory should still exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Path should still be a directory")
	}
}
