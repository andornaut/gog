package link

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/andornaut/gog/internal/repository"
)

// setupTestRepo creates a temporary test repository structure with git initialized
func setupTestRepo(t *testing.T) (repoPath string, cleanup func()) {
	tmpDir, err := os.MkdirTemp("", "gog-link-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	repoPath = filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	// Configure git for tests
	configCmds := [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
	}
	for _, args := range configCmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to configure git: %v", err)
		}
	}

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return repoPath, cleanup
}

// TestFileCreatesSymlink verifies basic symlink creation
func TestFileCreatesSymlink(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Set up home directory for testing
	testHome, err := os.MkdirTemp("", "gog-home-*")
	if err != nil {
		t.Fatalf("Failed to create test home: %v", err)
	}
	defer os.RemoveAll(testHome)

	originalHomeDir := repository.SetHomeDirForTest(testHome)
	defer func() { repository.SetHomeDirForTest(originalHomeDir) }()

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
	err = File(repoPath, intPath)
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
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Set up home directory for testing
	testHome, err := os.MkdirTemp("", "gog-home-*")
	if err != nil {
		t.Fatalf("Failed to create test home: %v", err)
	}
	defer os.RemoveAll(testHome)

	originalHomeDir := repository.SetHomeDirForTest(testHome)
	defer func() { repository.SetHomeDirForTest(originalHomeDir) }()

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
	err = File(repoPath, intPath)
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
	if isSymlink(extPath) {
		t.Error("Existing file should not have been replaced by a symlink")
	}
}

// TestFileHandlesBrokenSymlink verifies broken symlinks are replaced without backup
func TestFileHandlesBrokenSymlink(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Set up home directory for testing
	testHome, err := os.MkdirTemp("", "gog-home-*")
	if err != nil {
		t.Fatalf("Failed to create test home: %v", err)
	}
	defer os.RemoveAll(testHome)

	originalHomeDir := repository.SetHomeDirForTest(testHome)
	defer func() { repository.SetHomeDirForTest(originalHomeDir) }()

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
	err = File(repoPath, intPath)
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
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Set up home directory for testing
	testHome, err := os.MkdirTemp("", "gog-home-*")
	if err != nil {
		t.Fatalf("Failed to create test home: %v", err)
	}
	defer os.RemoveAll(testHome)

	originalHomeDir := repository.SetHomeDirForTest(testHome)
	defer func() { repository.SetHomeDirForTest(originalHomeDir) }()

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
	// Save and restore original regex
	originalRegex := ignoreFilesRegex
	defer func() { ignoreFilesRegex = originalRegex }()

	// Set ignore pattern to skip .swp files
	ignoreFilesRegex = regexp.MustCompile(`\.swp$`)

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Set up home directory for testing
	testHome, err := os.MkdirTemp("", "gog-home-*")
	if err != nil {
		t.Fatalf("Failed to create test home: %v", err)
	}
	defer os.RemoveAll(testHome)

	originalHomeDir := repository.SetHomeDirForTest(testHome)
	defer func() { repository.SetHomeDirForTest(originalHomeDir) }()

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
	err = File(repoPath, intPath)
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
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

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
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Set up home directory for testing
	testHome, err := os.MkdirTemp("", "gog-home-*")
	if err != nil {
		t.Fatalf("Failed to create test home: %v", err)
	}
	defer os.RemoveAll(testHome)

	originalHomeDir := repository.SetHomeDirForTest(testHome)
	defer func() { repository.SetHomeDirForTest(originalHomeDir) }()

	// Create a test file in the repo
	intPath := filepath.Join(repoPath, "$HOME", ".config")
	if err = os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err = os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create directory at external path (conflict)
	if err = os.MkdirAll(extPath, 0755); err != nil {
		t.Fatalf("Failed to create conflicting directory: %v", err)
	}

	// Attempt to create symlink. The conflict is reported and the run
	// continues, but the caller must still learn that it was incomplete.
	err = File(repoPath, intPath)
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

// TestIsSymlink verifies symlink detection
func TestIsSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-symlink-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create regular file
	regularFile := filepath.Join(tmpDir, "regular")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Create symlink
	symlinkPath := filepath.Join(tmpDir, "symlink")
	if err := os.Symlink(regularFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"regular file", regularFile, false},
		{"symlink", symlinkPath, true},
		{"nonexistent", filepath.Join(tmpDir, "nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSymlink(tt.path)
			if result != tt.expected {
				t.Errorf("isSymlink(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
