package copy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileCopiesToDestination verifies basic file copying
func TestFileCopiesToDestination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	testContent := []byte("test content for copy")
	if writeErr := os.WriteFile(srcPath, testContent, 0644); writeErr != nil {
		t.Fatalf("Failed to create source file: %v", writeErr)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dest.txt")
	err = File(srcPath, dstPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify destination file exists with correct content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != string(testContent) {
		t.Errorf("Destination content = %q, want %q", dstContent, testContent)
	}
}

// TestFilePreservesPermissions verifies file permissions are preserved
func TestFilePreservesPermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file with specific permissions
	srcPath := filepath.Join(tmpDir, "source.txt")
	if writeErr := os.WriteFile(srcPath, []byte("test"), 0600); writeErr != nil {
		t.Fatalf("Failed to create source file: %v", writeErr)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dest.txt")
	err = File(srcPath, dstPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify permissions are preserved
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("Failed to stat source: %v", err)
	}

	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("Failed to stat destination: %v", err)
	}

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("Permissions not preserved: src=%v, dst=%v", srcInfo.Mode(), dstInfo.Mode())
	}
}

// TestFileOverwritesExisting verifies existing files are overwritten
func TestFileOverwritesExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	newContent := []byte("new content")
	if writeErr := os.WriteFile(srcPath, newContent, 0644); writeErr != nil {
		t.Fatalf("Failed to create source file: %v", writeErr)
	}

	// Create existing destination file
	dstPath := filepath.Join(tmpDir, "dest.txt")
	oldContent := []byte("old content")
	if writeErr := os.WriteFile(dstPath, oldContent, 0644); writeErr != nil {
		t.Fatalf("Failed to create destination file: %v", writeErr)
	}

	// Copy file (should overwrite)
	err = File(srcPath, dstPath)
	if err != nil {
		t.Fatalf("File() failed: %v", err)
	}

	// Verify destination has new content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if string(dstContent) != string(newContent) {
		t.Errorf("Destination not overwritten: got %q, want %q", dstContent, newContent)
	}
}

// TestFileFailsForNonexistentSource verifies error when source doesn't exist
func TestFileFailsForNonexistentSource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	err = File(srcPath, dstPath)
	if err == nil {
		t.Error("File() should return error for nonexistent source")
	}
}

// TestDirCopiesRecursively verifies directory tree copying
func TestDirCopiesRecursively(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory structure
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}

	// Create files in structure
	files := map[string]string{
		"file1.txt":               "content 1",
		"subdir/file2.txt":        "content 2",
		"subdir/nested/file3.txt": "content 3",
	}

	for name, content := range files {
		path := filepath.Join(srcDir, name)
		if mkdirErr := os.MkdirAll(filepath.Dir(path), 0755); mkdirErr != nil {
			t.Fatalf("Failed to create dir: %v", mkdirErr)
		}
		if writeErr := os.WriteFile(path, []byte(content), 0644); writeErr != nil {
			t.Fatalf("Failed to create file %s: %v", name, writeErr)
		}
	}

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dst")
	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	// Verify all files were copied
	for name, expectedContent := range files {
		dstPath := filepath.Join(dstDir, name)
		content, err := os.ReadFile(dstPath)
		if err != nil {
			t.Errorf("Failed to read %s: %v", name, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, content, expectedContent)
		}
	}
}

// TestDirSkipsFunctionWorks verifies skip function is respected
func TestDirSkipsFunctionWorks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with files
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}

	// Create files
	keepFile := filepath.Join(srcDir, "keep.txt")
	skipFile := filepath.Join(srcDir, "skip.txt")

	if writeErr := os.WriteFile(keepFile, []byte("keep"), 0644); writeErr != nil {
		t.Fatalf("Failed to create keep file: %v", writeErr)
	}
	if writeErr := os.WriteFile(skipFile, []byte("skip"), 0644); writeErr != nil {
		t.Fatalf("Failed to create skip file: %v", writeErr)
	}

	// Copy directory with skip function
	dstDir := filepath.Join(tmpDir, "dst")
	skipFunc := func(src, dst string) bool {
		return filepath.Base(src) == "skip.txt"
	}

	err = Dir(srcDir, dstDir, skipFunc)
	if err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	// Verify keep.txt was copied
	dstKeep := filepath.Join(dstDir, "keep.txt")
	if _, err := os.Stat(dstKeep); err != nil {
		t.Error("keep.txt should be copied")
	}

	// Verify skip.txt was NOT copied
	dstSkip := filepath.Join(dstDir, "skip.txt")
	if _, err := os.Stat(dstSkip); !os.IsNotExist(err) {
		t.Error("skip.txt should not be copied")
	}
}

// TestDirFailsForNonDirectory verifies error when source is not a directory
func TestDirFailsForNonDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a regular file
	srcFile := filepath.Join(tmpDir, "file.txt")
	if writeErr := os.WriteFile(srcFile, []byte("test"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")

	// Attempt to copy file as directory
	err = Dir(srcFile, dstDir, func(src, dst string) bool { return false })
	if err == nil {
		t.Error("Dir() should return error when source is not a directory")
	}
}

// TestDirPreservesPermissions verifies directory permissions are preserved
func TestDirPreservesPermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with specific permissions
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.Mkdir(srcDir, 0700); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dst")
	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	// Verify permissions are preserved
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		t.Fatalf("Failed to stat source: %v", err)
	}

	dstInfo, err := os.Stat(dstDir)
	if err != nil {
		t.Fatalf("Failed to stat destination: %v", err)
	}

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("Directory permissions not preserved: src=%v, dst=%v", srcInfo.Mode(), dstInfo.Mode())
	}
}

// TestDirHandlesSymlinks verifies symlinks are resolved and copied
func TestDirHandlesSymlinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}

	// Create target file for symlink
	targetFile := filepath.Join(tmpDir, "target.txt")
	testContent := []byte("symlink target content")
	if writeErr := os.WriteFile(targetFile, testContent, 0644); writeErr != nil {
		t.Fatalf("Failed to create target file: %v", writeErr)
	}

	// Create symlink in source directory
	symlinkPath := filepath.Join(srcDir, "link.txt")
	if symlinkErr := os.Symlink(targetFile, symlinkPath); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dst")
	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	// Verify symlink was resolved and content copied
	dstFile := filepath.Join(dstDir, "link.txt")
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Symlink content not copied correctly: got %q, want %q", content, testContent)
	}
}

func TestDirHandlesSymlinkToDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}

	// Create target directory containing a file
	targetDir := filepath.Join(tmpDir, "targetdir")
	if mkdirErr := os.MkdirAll(targetDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create target dir: %v", mkdirErr)
	}
	testContent := []byte("file in symlinked directory")
	if writeErr := os.WriteFile(filepath.Join(targetDir, "file.txt"), testContent, 0644); writeErr != nil {
		t.Fatalf("Failed to create file in target dir: %v", writeErr)
	}

	// Create symlink to the directory in the source directory
	symlinkPath := filepath.Join(srcDir, "linkdir")
	if symlinkErr := os.Symlink(targetDir, symlinkPath); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dst")
	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	// Verify the symlinked directory was copied as a directory
	content, err := os.ReadFile(filepath.Join(dstDir, "linkdir", "file.txt"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Symlinked directory content not copied correctly: got %q, want %q", content, testContent)
	}
}

func TestDirRejectsSymlinkCycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a source directory containing a symlink to itself
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	if symlinkErr := os.Symlink(srcDir, filepath.Join(srcDir, "loop")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err == nil {
		t.Fatal("Dir() should fail on a symlink cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("Error should mention the cycle, got: %v", err)
	}
}

func TestDirRejectsAncestorSymlinkBeforeCopying(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// sibling would be copied in full if the cycle were only detected on re-entry
	if writeErr := os.WriteFile(filepath.Join(tmpDir, "sibling.txt"), []byte("x"), 0644); writeErr != nil {
		t.Fatalf("Failed to create sibling file: %v", writeErr)
	}
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	// Symlink to an ancestor of the source directory
	if symlinkErr := os.Symlink(tmpDir, filepath.Join(srcDir, "up")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err == nil {
		t.Fatal("Dir() should fail on a symlink to an ancestor")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("Error should mention the cycle, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "up", "sibling.txt")); statErr == nil {
		t.Error("The ancestor's contents should not have been copied before the cycle was detected")
	}
}

func TestDirRejectsSourceInsideDestination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	// Symlink into the destination tree
	if symlinkErr := os.Symlink(dstDir, filepath.Join(srcDir, "link")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err == nil {
		t.Fatal("Dir() should fail when a source entry resolves inside the destination")
	}
	if !strings.Contains(err.Error(), "destination") {
		t.Errorf("Error should mention the destination, got: %v", err)
	}
}
