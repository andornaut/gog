package link

import (
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
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
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

// TestFileBacksUpExistingFile verifies backup creation when file exists
func TestFileBacksUpExistingFile(t *testing.T) {
	// Temporarily enable backups for this test
	originalBackupDisabled := backupDisabled
	backupDisabled = false
	defer func() { backupDisabled = originalBackupDisabled }()

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
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(intPath, []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create existing file at external path
	if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
	}
	existingContent := []byte("existing content")
	if err := os.WriteFile(extPath, existingContent, 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Create symlink (should backup existing file)
	err = File(repoPath, intPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify backup was created
	backupPath := backupPath(extPath)
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Backup file not created: %v", err)
	}

	if string(backupContent) != string(existingContent) {
		t.Errorf("Backup content = %q, want %q", backupContent, existingContent)
	}

	// Verify symlink was created
	if _, err := os.Readlink(extPath); err != nil {
		t.Errorf("Symlink not created after backup: %v", err)
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
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create broken symlink at external path
	if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
	}
	brokenTarget := filepath.Join(repoPath, "nonexistent")
	if err := os.Symlink(brokenTarget, extPath); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
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

	// Verify no backup was created (broken symlinks don't get backed up)
	backupPath := backupPath(extPath)
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("Backup should not be created for broken symlinks")
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
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create correct symlink
	if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
	}
	if err := os.Symlink(intPath, extPath); err != nil {
		t.Fatalf("Failed to create initial symlink: %v", err)
	}

	// Get initial stat info
	initialInfo, err := os.Lstat(extPath)
	if err != nil {
		t.Fatalf("Failed to stat symlink: %v", err)
	}

	// Call File() again (should be no-op)
	err = File(repoPath, intPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify symlink wasn't modified (same inode, modtime, etc)
	finalInfo, err := os.Lstat(extPath)
	if err != nil {
		t.Fatalf("Failed to stat symlink after: %v", err)
	}

	if !os.SameFile(initialInfo, finalInfo) {
		t.Error("Symlink should not be modified when already correct")
	}
}

// TestFileSkipsIgnoredFiles verifies GOG_IGNORE_FILES_REGEX pattern matching
func TestFileSkipsIgnoredFiles(t *testing.T) {
	// Save and restore original regex
	originalRegex := ignoreFilesRegex
	defer func() { ignoreFilesRegex = originalRegex }()

	// Set ignore pattern to skip .swp files
	var err error
	ignoreFilesRegex, err = regexp.Compile(`\.swp$`)
	if err != nil {
		t.Fatalf("Failed to compile test regex: %v", err)
	}

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
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
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

// TestFileSkipsExistingDirectory verifies error when directory exists
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

	// Attempt to create symlink (should print error and return nil)
	err = File(repoPath, intPath)
	if err != nil {
		t.Fatalf("File() should return nil for directory conflicts, got: %v", err)
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

// TestBackupPath verifies correct backup filename generation
func TestBackupPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/home/user/.bashrc",
			expected: "/home/user/.bashrc.gog",
		},
		{
			input:    "/home/user/config",
			expected: "/home/user/.config.gog",
		},
		{
			input:    "/etc/hosts",
			expected: "/etc/.hosts.gog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := backupPath(tt.input)
			if result != tt.expected {
				t.Errorf("backupPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
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
