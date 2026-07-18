package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestIs ensures that only the root of a git repository is recognized,
// not its subdirectories or plain directories
func TestIs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoPath := filepath.Join(tmpDir, "repo")
	subPath := filepath.Join(repoPath, "sub")
	plainPath := filepath.Join(tmpDir, "plain")
	for _, p := range []string{subPath, plainPath} {
		if mkdirErr := os.MkdirAll(p, 0755); mkdirErr != nil {
			t.Fatalf("Failed to create dir %s: %v", p, mkdirErr)
		}
	}

	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repoPath
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("Failed to initialize git repo: %v", runErr)
	}

	if !Is(repoPath) {
		t.Error("Is() should return true for the root of a git repository")
	}
	if Is(subPath) {
		t.Error("Is() should return false for a subdirectory of a git repository")
	}
	if Is(plainPath) {
		t.Error("Is() should return false for a directory that is not a git repository")
	}

	barePath := filepath.Join(tmpDir, "bare.git")
	cmd = exec.Command("git", "init", "-q", "--bare", barePath)
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("Failed to initialize bare git repo: %v", runErr)
	}
	if Is(barePath) {
		t.Error("Is() should return false for a bare repository (no work tree to link from)")
	}

	// GIT_DIR must not leak into git invocations (e.g. when run from a git hook)
	t.Setenv("GIT_DIR", filepath.Join(repoPath, ".git"))
	if Is(plainPath) {
		t.Error("Is() should return false for a non-repository directory when GIT_DIR is set")
	}
	if !Is(repoPath) {
		t.Error("Is() should return true for a repository root when GIT_DIR is set")
	}
}
