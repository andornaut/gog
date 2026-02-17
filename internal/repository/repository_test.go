package repository

import (
	"os"
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
		if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0755); err != nil {
			t.Fatalf("Failed to create test repo: %v", err)
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
	if err := os.MkdirAll(filepath.Join(outsideDir, ".git"), 0755); err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	// Should reject directory traversal
	_, err = RootPath("../outside-gog")
	if err == nil {
		t.Error("RootPath should reject paths outside BaseDir")
	}
}
