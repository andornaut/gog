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
		// A base that was never resolved holds nothing. Read as a prefix, an
		// empty one would contain every absolute path.
		{"", "/home/alice/.bashrc", false},
		{"", "/", false},
		{"", "", false},
		{"", "relative", false},
	}
	for _, tt := range tests {
		if got := Within(tt.base, tt.p); got != tt.want {
			t.Errorf("Within(%q, %q) = %v, want %v", tt.base, tt.p, got, tt.want)
		}
	}
}

// A path that does not exist yet is resolved as far as it does, so that a
// destination under a relocated directory is compared in the same terms as
// everything else
func TestResolveExistingPrefix(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(root, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}
	// realDir may itself contain symbolic link components, so resolve it for
	// the comparison
	resolvedReal, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatal(err)
	}

	got := Resolve(filepath.Join(linkDir, "child", "grandchild"))

	if want := filepath.Join(resolvedReal, "child", "grandchild"); got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
}

func TestIsSymlink(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	if err := os.WriteFile(regular, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
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
