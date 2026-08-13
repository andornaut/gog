package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithin(t *testing.T) {
	tests := []struct {
		base string
		p    string
		want bool
	}{
		{"/home/alice", "/home/alice", true},
		{"/home/alice", "/home/alice/.bashrc", true},
		{"/home/alice", "/home/alicebob", false},
		{"/home/alice", "/home/alicebob/.bashrc", false},
		{"/home/alice", "/etc/passwd", false},
		{"/", "/etc/passwd", true},
		{"/", "/", true},
		{"/home/alice/", "/home/alice/.bashrc", true},
	}
	for _, tt := range tests {
		if got := Within(tt.base, tt.p); got != tt.want {
			t.Errorf("Within(%q, %q) = %v, want %v", tt.base, tt.p, got, tt.want)
		}
	}
}

func TestResolveExistingPrefix(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gog-paths-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	realDir := filepath.Join(tmpDir, "real")
	if mkdirErr := os.MkdirAll(realDir, 0755); mkdirErr != nil {
		t.Fatalf("Failed to create real dir: %v", mkdirErr)
	}
	linkDir := filepath.Join(tmpDir, "link")
	if symlinkErr := os.Symlink(realDir, linkDir); symlinkErr != nil {
		t.Fatalf("Failed to create symlink: %v", symlinkErr)
	}

	// A non-existent path under a symlinked, existing prefix resolves the
	// prefix and keeps the remainder. realDir itself may contain symlink
	// components (e.g. /var on macOS), so resolve it for the comparison.
	resolvedReal, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatalf("Failed to resolve real dir: %v", err)
	}
	got := Resolve(filepath.Join(linkDir, "child", "grandchild"))
	want := filepath.Join(resolvedReal, "child", "grandchild")
	if got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
}

// A base that was never resolved holds nothing. Without this it reads as a
// prefix of every absolute path, and every path looks like it is inside it.
func TestWithinEmptyBase(t *testing.T) {
	for _, p := range []string{"/home/alice/.bashrc", "/", "", "relative"} {
		if Within("", p) {
			t.Errorf("Within(\"\", %q) = true, want false", p)
		}
	}
}

func TestIsSymlink(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	if err := os.WriteFile(regular, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "symlink")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "broken")
	if err := os.Symlink(filepath.Join(root, "gone"), broken); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "a regular file", path: regular},
		{name: "a symbolic link", path: link, want: true},
		{name: "a broken symbolic link is still a link", path: broken, want: true},
		{name: "a path that does not exist", path: filepath.Join(root, "gone")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSymlink(tt.path); got != tt.want {
				t.Errorf("IsSymlink(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
