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

	// Create source directory with specific permissions. It needs a file
	// because a directory with nothing to copy is not created.
	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.Mkdir(srcDir, 0700); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("x"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
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

// TestDirSkipsEmptyDirectories verifies that a directory with nothing to copy
// into it is not created at the destination, so that an empty directory does
// not become an untrackable entry in the repository
func TestDirSkipsEmptyDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(filepath.Join(srcDir, "empty", "alsoempty"), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create empty dirs: %v", mkdirErr)
	}
	if mkdirErr := os.MkdirAll(filepath.Join(srcDir, "full"), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create full dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(srcDir, "full", "file.txt"), []byte("x"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dstDir, "full", "file.txt")); statErr != nil {
		t.Errorf("The non-empty directory should have been copied: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "empty")); !os.IsNotExist(statErr) {
		t.Error("An empty directory should not be created at the destination")
	}
}

// TestDirSkipsEntirelyEmptySource verifies that copying a source that holds
// nothing creates no destination at all
func TestDirSkipsEntirelyEmptySource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.Mkdir(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}
	if _, statErr := os.Stat(dstDir); !os.IsNotExist(statErr) {
		t.Error("An empty source should not create a destination directory")
	}
}

// TestDirSkipsSymlinkToFile verifies that a symlink is left behind rather than
// replaced by a copy of its target's contents
func TestDirSkipsSymlinkToFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}

	// Create target file for symlink, within the source tree
	targetFile := filepath.Join(srcDir, "target.txt")
	if writeErr := os.WriteFile(targetFile, []byte("symlink target content"), 0644); writeErr != nil {
		t.Fatalf("Failed to create target file: %v", writeErr)
	}
	if symlinkErr := os.Symlink(targetFile, filepath.Join(srcDir, "link.txt")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	// The target itself is a regular file and is copied
	if _, statErr := os.Stat(filepath.Join(dstDir, "target.txt")); statErr != nil {
		t.Errorf("The symlink's target should have been copied: %v", statErr)
	}
	// The symlink is not, in either form
	if _, statErr := os.Lstat(filepath.Join(dstDir, "link.txt")); !os.IsNotExist(statErr) {
		t.Error("The symlink should not have been copied")
	}
}

// TestDirSkipsSymlinkToDirectory verifies that a symlink to a directory is
// skipped rather than copied as a directory
func TestDirSkipsSymlinkToDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	targetDir := filepath.Join(srcDir, "targetdir")
	if mkdirErr := os.MkdirAll(targetDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create target dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("x"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file in target dir: %v", writeErr)
	}
	if symlinkErr := os.Symlink(targetDir, filepath.Join(srcDir, "linkdir")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dstDir, "targetdir", "file.txt")); statErr != nil {
		t.Errorf("The real directory should have been copied: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(dstDir, "linkdir")); !os.IsNotExist(statErr) {
		t.Error("The symlinked directory should not have been copied")
	}
}

// TestDirSkipsBrokenSymlink verifies that a symlink whose target is missing is
// skipped rather than failing the whole copy
func TestDirSkipsBrokenSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("keep"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}
	if symlinkErr := os.Symlink(filepath.Join(tmpDir, "gone"), filepath.Join(srcDir, "broken")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() should skip a broken symlink, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "keep.txt")); statErr != nil {
		t.Errorf("The rest of the directory should have been copied: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(dstDir, "broken")); !os.IsNotExist(statErr) {
		t.Error("The broken symlink should not have been copied")
	}
}

// TestDirSkipsSelfReferentialSymlink verifies that a symlink pointing at its
// own directory is skipped, so no cycle can be entered
func TestDirSkipsSelfReferentialSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("keep"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}
	if symlinkErr := os.Symlink(srcDir, filepath.Join(srcDir, "loop")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() should skip a self-referential symlink, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "keep.txt")); statErr != nil {
		t.Errorf("The rest of the directory should have been copied: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(dstDir, "loop")); !os.IsNotExist(statErr) {
		t.Error("The self-referential symlink should not have been copied")
	}
}

// TestDirSkipsAncestorSymlink verifies that a symlink resolving to an ancestor
// directory within the source is skipped rather than followed back up the tree
func TestDirSkipsAncestorSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// src/a/b/loop -> src/a: the target is an ancestor but stays within src
	srcDir := filepath.Join(tmpDir, "src")
	deepDir := filepath.Join(srcDir, "a", "b")
	if mkdirErr := os.MkdirAll(deepDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dirs: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(deepDir, "keep.txt"), []byte("keep"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}
	if symlinkErr := os.Symlink(filepath.Join(srcDir, "a"), filepath.Join(deepDir, "loop")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() should skip an ancestor symlink, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "a", "b", "keep.txt")); statErr != nil {
		t.Errorf("The rest of the directory should have been copied: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(dstDir, "a", "b", "loop")); !os.IsNotExist(statErr) {
		t.Error("The ancestor symlink should not have been copied")
	}
}

// TestDirSkipsSymlinkEscapingSource verifies that a symlink whose target
// resolves outside the directory being copied is skipped, so that unrelated
// files are never pulled in
func TestDirSkipsSymlinkEscapingSource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// outside holds content that lives beyond the source tree
	outside := filepath.Join(tmpDir, "outside")
	if mkdirErr := os.MkdirAll(outside, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create outside dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0644); writeErr != nil {
		t.Fatalf("Failed to create outside file: %v", writeErr)
	}

	srcDir := filepath.Join(tmpDir, "src")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("keep"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}
	if symlinkErr := os.Symlink(outside, filepath.Join(srcDir, "escape")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err = Dir(srcDir, dstDir, func(src, dst string) bool { return false }); err != nil {
		t.Fatalf("Dir() should skip an escaping symlink, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "keep.txt")); statErr != nil {
		t.Errorf("The non-escaping file should have been copied: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "escape", "secret.txt")); statErr == nil {
		t.Error("The escaping symlink's target should not have been copied")
	}
}

// TestDirRejectsSourceInsideDestination ensures a source that lives inside the
// destination is rejected, because it would be re-copied into itself
func TestDirRejectsSourceInsideDestination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// The source is a subdirectory of the destination
	dstDir := filepath.Join(tmpDir, "dst")
	srcDir := filepath.Join(dstDir, "inner")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("x"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}

	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err == nil {
		t.Fatal("Dir() should fail when the source is nested inside the destination")
	}
	if !strings.Contains(err.Error(), "destination") {
		t.Errorf("Error should mention the destination, got: %v", err)
	}
}

// TestDirRejectsSourceInsideSymlinkedDestination ensures the overlap check
// resolves symlinks: the destination is addressed through a symlinked path
// component (e.g. a relocated ~/.local, or /var on macOS) but resolves to an
// ancestor of the source, so the overlap must still be detected
func TestDirRejectsSourceInsideSymlinkedDestination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-copy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// linkDir addresses realDir through a symlink, so the destination is only
	// seen to contain the source once it is resolved
	realDir := filepath.Join(tmpDir, "real")
	srcDir := filepath.Join(realDir, "inner")
	if mkdirErr := os.MkdirAll(srcDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create source dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("x"), 0644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}
	if symlinkErr := os.Symlink(realDir, filepath.Join(tmpDir, "link")); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	// dst resolves to realDir, i.e. an ancestor of the source
	dstDir := filepath.Join(tmpDir, "link")
	err = Dir(srcDir, dstDir, func(src, dst string) bool { return false })
	if err == nil {
		t.Fatal("Dir() should fail when a symlink-addressed destination contains the source")
	}
	if !strings.Contains(err.Error(), "destination") {
		t.Errorf("Error should mention the destination, got: %v", err)
	}
}
