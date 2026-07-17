package repository

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRootPathAmbiguousMatch tests critical security fix:
// ensures ambiguous repository names are rejected to prevent attacks
func TestRootPathAmbiguousMatch(t *testing.T) {
	originalBaseDir := BaseDir
	defer func() { BaseDir = originalBaseDir }()

	tmpDir, err := os.MkdirTemp("", "gog-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	BaseDir = tmpDir

	// Create multiple repositories with similar names
	for _, suffix := range []string{"-v1", "-v2"} {
		repoPath := filepath.Join(BaseDir, "myrepo"+suffix)
		if mkdirErr := os.MkdirAll(filepath.Join(repoPath, ".git"), 0755); mkdirErr != nil {
			t.Fatalf("Failed to create test repo: %v", mkdirErr)
		}
	}

	// Should reject ambiguous match
	_, err = RootPath("myrepo")
	if err == nil {
		t.Error("RootPath should return error for ambiguous match")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("Error should mention ambiguity, got: %v", err)
	}
}

// TestRootPathDirectoryTraversal tests critical security fix:
// ensures paths outside BaseDir are rejected
func TestRootPathDirectoryTraversal(t *testing.T) {
	originalBaseDir := BaseDir
	defer func() { BaseDir = originalBaseDir }()

	tmpDir, err := os.MkdirTemp("", "gog-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	BaseDir = tmpDir

	// Create a directory outside BaseDir
	outsideDir := filepath.Join(filepath.Dir(tmpDir), "outside-gog")
	if mkdirErr := os.MkdirAll(filepath.Join(outsideDir, ".git"), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create outside dir: %v", mkdirErr)
	}
	defer os.RemoveAll(outsideDir)

	// Should reject directory traversal
	_, err = RootPath("../outside-gog")
	if err == nil {
		t.Error("RootPath should reject paths outside BaseDir")
	}
}

// TestGetFirstSkipsInvalidRepositories ensures the default repository is
// selected with the same validation as List(), skipping non-git directories
func TestGetFirstSkipsInvalidRepositories(t *testing.T) {
	originalBaseDir := BaseDir
	defer func() { BaseDir = originalBaseDir }()

	tmpDir, err := os.MkdirTemp("", "gog-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	BaseDir = tmpDir

	// No repositories yet
	if _, firstErr := getFirst(); firstErr == nil {
		t.Error("getFirst should return an error when there are no repositories")
	}

	// "aaa" sorts first, but is not a git repository
	if mkdirErr := os.MkdirAll(filepath.Join(BaseDir, "aaa"), 0755); mkdirErr != nil {
		t.Fatalf("Failed to create non-repo dir: %v", mkdirErr)
	}

	// "bbb" is a valid git repository
	repoPath := filepath.Join(BaseDir, "bbb")
	if mkdirErr := os.MkdirAll(repoPath, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create repo dir: %v", mkdirErr)
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repoPath
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("Failed to initialize git repo: %v", runErr)
	}

	got, err := getFirst()
	if err != nil {
		t.Fatalf("getFirst() failed: %v", err)
	}
	if got != repoPath {
		t.Errorf("getFirst() = %q, want %q", got, repoPath)
	}
}

// TestGetBaseDirNormalizesPath ensures GOG_HOME values with trailing slashes
// or relative paths are normalized to clean absolute paths
func TestGetBaseDirNormalizesPath(t *testing.T) {
	t.Setenv("GOG_HOME", "/data/gog/")
	got, err := getBaseDir("/home/testuser")
	if err != nil {
		t.Fatalf("getBaseDir() failed: %v", err)
	}
	if got != "/data/gog" {
		t.Errorf("getBaseDir() = %q, want %q", got, "/data/gog")
	}

	t.Setenv("GOG_HOME", "relative-gog")
	got, err = getBaseDir("/home/testuser")
	if err != nil {
		t.Fatalf("getBaseDir() failed: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("getBaseDir() = %q, want an absolute path", got)
	}
}
