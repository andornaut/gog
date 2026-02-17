package link

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/andornaut/gog/internal/repository"
)

// TestUnlinkFileRestoresFromSymlink verifies symlink is replaced with actual file
func TestUnlinkFileRestoresFromSymlink(t *testing.T) {
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
	testContent := []byte("test content for unlink")
	intPath := filepath.Join(repoPath, "$HOME", ".bashrc")
	if err = os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err = os.WriteFile(intPath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create symlink
	if err = os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
	}
	if err = os.Symlink(intPath, extPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Verify it's a symlink
	if !isSymlink(extPath) {
		t.Fatal("Expected symlink to be created")
	}

	// Add file to git (required for git rm to work)
	cmd := exec.Command("git", "add", "-f", intPath)
	cmd.Dir = repoPath
	if err = cmd.Run(); err != nil {
		t.Fatalf("Failed to add file to git: %v", err)
	}

	// Unlink the file
	err = UnlinkFile(repoPath, intPath)
	if err != nil {
		t.Fatalf("UnlinkFile() failed: %v", err)
	}

	// Verify symlink is replaced with regular file
	if isSymlink(extPath) {
		t.Error("Path should no longer be a symlink")
	}

	// Verify file contains correct content
	content, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatalf("Failed to read unlinked file: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Unlinked file content = %q, want %q", content, testContent)
	}
}

// TestUnlinkFileSkipsNonSymlink verifies no-op when path is not a symlink
func TestUnlinkFileSkipsNonSymlink(t *testing.T) {
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
	if err = os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err = os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create regular file (not a symlink)
	regularContent := []byte("regular file content")
	if err = os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
	}
	if err = os.WriteFile(extPath, regularContent, 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Attempt to unlink (should be no-op)
	err = UnlinkFile(repoPath, intPath)
	if err != nil {
		t.Fatalf("UnlinkFile() should return nil for non-symlinks, got: %v", err)
	}

	// Verify file is unchanged
	content, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != string(regularContent) {
		t.Error("Regular file should not be modified")
	}
}

// TestUnlinkFileSkipsWrongTarget verifies no-op when symlink points elsewhere
func TestUnlinkFileSkipsWrongTarget(t *testing.T) {
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
	if err = os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err = os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extPath := repository.ToExternalPath(repoPath, intPath)

	// Create a different target file
	otherTarget := filepath.Join(repoPath, "other-file")
	if err = os.WriteFile(otherTarget, []byte("other content"), 0644); err != nil {
		t.Fatalf("Failed to create other file: %v", err)
	}

	// Create symlink pointing to different file
	if err = os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
	}
	if err = os.Symlink(otherTarget, extPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Attempt to unlink (should be no-op because symlink points elsewhere)
	err = UnlinkFile(repoPath, intPath)
	if err != nil {
		t.Fatalf("UnlinkFile() should return nil for wrong target, got: %v", err)
	}

	// Verify symlink still exists and points to otherTarget
	if !isSymlink(extPath) {
		t.Error("Symlink should still exist")
	}

	target, err := os.Readlink(extPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	if target != otherTarget {
		t.Errorf("Symlink target changed, got %q, want %q", target, otherTarget)
	}
}

// TestUnlinkFileSkipsNonexistent verifies no-op when external file doesn't exist
func TestUnlinkFileSkipsNonexistent(t *testing.T) {
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
	if err = os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err = os.WriteFile(intPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Don't create external file - it doesn't exist

	// Attempt to unlink (should be no-op)
	err = UnlinkFile(repoPath, intPath)
	if err != nil {
		t.Fatalf("UnlinkFile() should return nil for nonexistent file, got: %v", err)
	}
}

// TestUnlinkDirProcessesAllFiles verifies directory unlinking is recursive
func TestUnlinkDirProcessesAllFiles(t *testing.T) {
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

	// Create test directory structure in repo
	testDir := filepath.Join(repoPath, "$HOME", ".config")
	if err = os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Create multiple files
	files := map[string]string{
		"file1.txt":               "content 1",
		"file2.txt":               "content 2",
		"subdir/file3.txt":        "content 3",
	}

	for name, content := range files {
		intPath := filepath.Join(testDir, name)
		if err = os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err = os.WriteFile(intPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", name, err)
		}
	}

	// Create symlinks for all files
	extBaseDir := repository.ToExternalPath(repoPath, testDir)
	for name := range files {
		intPath := filepath.Join(testDir, name)
		extPath := filepath.Join(extBaseDir, name)

		if err = os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
			t.Fatalf("Failed to create ext dir: %v", err)
		}
		if err = os.Symlink(intPath, extPath); err != nil {
			t.Fatalf("Failed to create symlink for %s: %v", name, err)
		}
	}

	// Add files to git (required for git rm to work)
	cmd := exec.Command("git", "add", "-f", testDir)
	cmd.Dir = repoPath
	if err = cmd.Run(); err != nil {
		t.Fatalf("Failed to add files to git: %v", err)
	}

	// Unlink the entire directory
	err = UnlinkDir(repoPath, testDir)
	if err != nil {
		t.Fatalf("UnlinkDir() failed: %v", err)
	}

	// Verify all symlinks were replaced with regular files
	for name, expectedContent := range files {
		extPath := filepath.Join(extBaseDir, name)

		if isSymlink(extPath) {
			t.Errorf("File %s should no longer be a symlink", name)
		}

		content, err := os.ReadFile(extPath)
		if err != nil {
			t.Errorf("Failed to read %s: %v", name, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, content, expectedContent)
		}
	}
}
