package fscopy

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// copyNothing is a SkipFunc that skips nothing, so that a test exercises only
// what Dir itself decides
func copyNothing(_, _ string) bool { return false }

// reported collects the entries a copy left behind
type reported struct {
	paths []string
	modes []os.FileMode
}

func (r *reported) record(p string, mode os.FileMode) {
	r.paths = append(r.paths, p)
	r.modes = append(r.modes, mode)
}

// reportNothing discards what a copy left behind, for a test that asserts only
// what reached the destination
func reportNothing(_ string, _ os.FileMode) {}

func write(t *testing.T, p, contents string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func assertContents(t *testing.T, p, want string) {
	t.Helper()
	contents, err := os.ReadFile(p)
	if err != nil || string(contents) != want {
		t.Errorf("%s holds %q (%v), want %q", p, contents, err, want)
	}
}

func assertAbsent(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Lstat(p); !os.IsNotExist(err) {
		t.Errorf("%s exists (%v), want nothing", p, err)
	}
}

func TestFileCopiesContentsAndMode(t *testing.T) {
	root := t.TempDir()
	src := write(t, filepath.Join(root, "source"), "contents\n", 0600)
	dst := filepath.Join(root, "destination")

	if err := File(src, dst); err != nil {
		t.Fatalf("File() = %v", err)
	}

	assertContents(t, dst, "contents\n")
	info, err := os.Stat(dst)
	if err != nil || info.Mode() != 0600 {
		t.Errorf("%s has mode %v (%v), want 0600", dst, info.Mode(), err)
	}
}

func TestFileReplacesTheDestination(t *testing.T) {
	root := t.TempDir()
	src := write(t, filepath.Join(root, "source"), "new\n", 0644)
	dst := write(t, filepath.Join(root, "destination"), "old\n", 0644)

	if err := File(src, dst); err != nil {
		t.Fatalf("File() = %v", err)
	}

	assertContents(t, dst, "new\n")
}

func TestFileFailsWhenTheSourceDoesNotExist(t *testing.T) {
	root := t.TempDir()

	if err := File(filepath.Join(root, "gone"), filepath.Join(root, "destination")); err == nil {
		t.Error("File() reported success for a source that does not exist")
	}
}

func TestDirCopiesATreeAndItsModes(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(src, "one"), "one\n", 0644)
	write(t, filepath.Join(src, "sub", "two"), "two\n", 0644)
	write(t, filepath.Join(src, "sub", "nested", "three"), "three\n", 0644)
	dst := filepath.Join(root, "dst")

	if err := Dir(src, dst, copyNothing, reportNothing); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	assertContents(t, filepath.Join(dst, "one"), "one\n")
	assertContents(t, filepath.Join(dst, "sub", "two"), "two\n")
	assertContents(t, filepath.Join(dst, "sub", "nested", "three"), "three\n")
	info, err := os.Stat(filepath.Join(dst, "sub"))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Errorf("sub has mode %v (%v), want 0700", info.Mode().Perm(), err)
	}
}

func TestDirHonoursTheSkipFunc(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	write(t, filepath.Join(src, "keep"), "keep\n", 0644)
	write(t, filepath.Join(src, "skip"), "skip\n", 0644)
	dst := filepath.Join(root, "dst")

	err := Dir(src, dst, func(srcPath, _ string) bool {
		return filepath.Base(srcPath) == "skip"
	}, reportNothing)

	if err != nil {
		t.Fatalf("Dir() = %v", err)
	}
	assertContents(t, filepath.Join(dst, "keep"), "keep\n")
	assertAbsent(t, filepath.Join(dst, "skip"))
}

func TestDirFailsWhenTheSourceIsNotADirectory(t *testing.T) {
	root := t.TempDir()
	src := write(t, filepath.Join(root, "file"), "x\n", 0644)

	if err := Dir(src, filepath.Join(root, "dst"), copyNothing, reportNothing); err == nil {
		t.Error("Dir() reported success for a source that is not a directory")
	}
}

// A directory that holds nothing to copy leaves no trace at the destination,
// so that an empty directory does not become an entry git cannot track
func TestDirCreatesNoDirectoryWithNothingToHold(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "empty", "alsoempty"), 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(src, "full", "file"), "file\n", 0644)
	dst := filepath.Join(root, "dst")

	if err := Dir(src, dst, copyNothing, reportNothing); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	assertContents(t, filepath.Join(dst, "full", "file"), "file\n")
	assertAbsent(t, filepath.Join(dst, "empty"))

	// A source that holds nothing at all creates no destination either
	emptySrc := filepath.Join(root, "empty-src")
	if err := os.Mkdir(emptySrc, 0755); err != nil {
		t.Fatal(err)
	}
	emptyDst := filepath.Join(root, "empty-dst")

	if err := Dir(emptySrc, emptyDst, copyNothing, reportNothing); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	assertAbsent(t, emptyDst)
}

// A named pipe, socket or device node is skipped rather than opened, which
// lets a directory such as ~/.gnupg be copied while the sockets in it are left
// alone
func TestDirSkipsAnIrregularFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	write(t, filepath.Join(src, "keep"), "keep\n", 0644)
	listener, err := net.Listen("unix", filepath.Join(src, "socket"))
	if err != nil {
		t.Skipf("cannot create a unix socket: %v", err)
	}
	defer func() { _ = listener.Close() }()
	dst := filepath.Join(root, "dst")
	socket := filepath.Join(src, "socket")

	var left reported
	if err := Dir(src, dst, copyNothing, left.record); err != nil {
		t.Fatalf("Dir() = %v", err)
	}

	assertContents(t, filepath.Join(dst, "keep"), "keep\n")
	assertAbsent(t, filepath.Join(dst, "socket"))
	if !slices.Equal(left.paths, []string{socket}) {
		t.Errorf("Dir() reported %q, want %q", left.paths, []string{socket})
	}
	if len(left.modes) != 1 || left.modes[0]&os.ModeSocket == 0 {
		t.Errorf("Dir() reported mode %v, want a socket", left.modes)
	}
}

// A symbolic link is skipped rather than followed: copying its target would
// store the contents while discarding the link, and because none is followed
// the walk cannot meet a cycle or a path outside the tree
func TestDirSkipsSymbolicLinks(t *testing.T) {
	tests := []struct {
		name string
		// prepare creates the link and returns its path relative to the source
		prepare func(t *testing.T, root, src string) string
		// alsoCopied is a source-relative path that must reach the destination
		alsoCopied string
	}{
		{
			name: "to a file",
			prepare: func(t *testing.T, _, src string) string {
				t.Helper()
				link(t, write(t, filepath.Join(src, "target"), "target\n", 0644), filepath.Join(src, "link"))
				return "link"
			},
			alsoCopied: "target",
		},
		{
			name: "to a directory",
			prepare: func(t *testing.T, _, src string) string {
				t.Helper()
				write(t, filepath.Join(src, "targetdir", "file"), "file\n", 0644)
				link(t, filepath.Join(src, "targetdir"), filepath.Join(src, "linkdir"))
				return "linkdir"
			},
			alsoCopied: "targetdir/file",
		},
		{
			name: "whose target is missing",
			prepare: func(t *testing.T, root, src string) string {
				t.Helper()
				link(t, filepath.Join(root, "gone"), filepath.Join(src, "broken"))
				return "broken"
			},
		},
		{
			name: "to its own directory",
			prepare: func(t *testing.T, _, src string) string {
				t.Helper()
				link(t, src, filepath.Join(src, "loop"))
				return "loop"
			},
		},
		{
			name: "to an ancestor within the source",
			prepare: func(t *testing.T, _, src string) string {
				t.Helper()
				write(t, filepath.Join(src, "a", "b", "file"), "file\n", 0644)
				link(t, filepath.Join(src, "a"), filepath.Join(src, "a", "b", "loop"))
				return "a/b/loop"
			},
			alsoCopied: "a/b/file",
		},
		{
			name: "to a path outside the source",
			prepare: func(t *testing.T, root, src string) string {
				t.Helper()
				write(t, filepath.Join(root, "outside", "elsewhere"), "elsewhere\n", 0644)
				link(t, filepath.Join(root, "outside"), filepath.Join(src, "escape"))
				return "escape"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "src")
			write(t, filepath.Join(src, "keep"), "keep\n", 0644)
			rel := tt.prepare(t, root, src)
			dst := filepath.Join(root, "dst")

			var left reported
			if err := Dir(src, dst, copyNothing, left.record); err != nil {
				t.Fatalf("Dir() = %v", err)
			}

			assertAbsent(t, filepath.Join(dst, rel))
			// The link is reported rather than passed over in silence, and the
			// rest of the tree is still copied
			if want := []string{filepath.Join(src, rel)}; !slices.Equal(left.paths, want) {
				t.Errorf("Dir() reported %q, want %q", left.paths, want)
			}
			assertContents(t, filepath.Join(dst, "keep"), "keep\n")
			if tt.alsoCopied != "" {
				if _, err := os.Stat(filepath.Join(dst, tt.alsoCopied)); err != nil {
					t.Errorf("%s did not reach the destination: %v", tt.alsoCopied, err)
				}
			}
		})
	}
}

// A source inside the destination would be copied into itself, and the two are
// compared resolved because either may be addressed through a symbolic link
func TestDirRefusesASourceInsideTheDestination(t *testing.T) {
	tests := []struct {
		name string
		// prepare returns the source and destination to copy between
		prepare func(t *testing.T, root string) (src, dst string)
	}{
		{
			name: "named directly",
			prepare: func(t *testing.T, root string) (string, string) {
				t.Helper()
				dst := filepath.Join(root, "dst")
				src := filepath.Join(dst, "inner")
				write(t, filepath.Join(src, "file"), "file\n", 0644)
				return src, dst
			},
		},
		{
			name: "reached through a symbolic link",
			prepare: func(t *testing.T, root string) (string, string) {
				t.Helper()
				realDir := filepath.Join(root, "real")
				src := filepath.Join(realDir, "inner")
				write(t, filepath.Join(src, "file"), "file\n", 0644)
				dst := filepath.Join(root, "link")
				link(t, realDir, dst)
				return src, dst
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst := tt.prepare(t, t.TempDir())

			err := Dir(src, dst, copyNothing, reportNothing)

			if err == nil || !strings.Contains(err.Error(), "destination") {
				t.Errorf("Dir() = %v, want a failure naming the destination", err)
			}
		})
	}
}

func link(t *testing.T, target, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
}
