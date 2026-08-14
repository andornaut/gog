package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andornaut/gog/internal/paths"
	"github.com/andornaut/gog/internal/repository"
)

func TestUnlinkFileRestoresWhatTheLinkPointedAt(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	extPath := filepath.Join(homeDir, ".bashrc")
	if err := os.Symlink(intPath, extPath); err != nil {
		t.Fatal(err)
	}

	if err := UnlinkFile(repoPath, intPath); err != nil {
		t.Fatalf("UnlinkFile() = %v", err)
	}

	if paths.IsSymlink(extPath) {
		t.Errorf("%s is still a symbolic link", extPath)
	}
	if contents, err := os.ReadFile(extPath); err != nil || string(contents) != "bashrc\n" {
		t.Errorf("%s holds %q (%v), want the repository's contents", extPath, contents, err)
	}
}

// The result line names the path that was given back
func TestUnlinkFilePrintsWhatItRestored(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	extPath := filepath.Join(homeDir, ".bashrc")
	if err := os.Symlink(intPath, extPath); err != nil {
		t.Fatal(err)
	}

	out := captureStderr(t, func() {
		if err := UnlinkFile(repoPath, intPath); err != nil {
			t.Errorf("UnlinkFile() = %v", err)
		}
	})

	if want := "Restored: " + extPath + "\n"; out != want {
		t.Errorf("UnlinkFile() printed %q, want %q", out, want)
	}
}

// Anything that is not this repository's link is left exactly as it is
func TestUnlinkFileLeavesAloneWhatIsNotItsLink(t *testing.T) {
	tests := []struct {
		name string
		// prepare puts something at extPath, or nothing when it is nil
		prepare  func(t *testing.T, extPath string)
		want     string
		wantLink bool
	}{
		{
			name: "a file of the user's",
			prepare: func(t *testing.T, extPath string) {
				if err := os.WriteFile(extPath, []byte("mine\n"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			want: "mine\n",
		},
		{
			name: "a link to another repository's copy",
			prepare: func(t *testing.T, extPath string) {
				other := write(t, filepath.Join(repository.BaseDir, "other"), "$HOME/.bashrc", "theirs\n")
				if err := os.Symlink(other, extPath); err != nil {
					t.Fatal(err)
				}
			},
			want:     "theirs\n",
			wantLink: true,
		},
		{name: "nothing at all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, homeDir := newSandbox(t)
			intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
			extPath := filepath.Join(homeDir, ".bashrc")
			if tt.prepare != nil {
				tt.prepare(t, extPath)
			}

			if err := UnlinkFile(repoPath, intPath); err != nil {
				t.Fatalf("UnlinkFile() = %v", err)
			}

			contents, err := os.ReadFile(extPath)
			if tt.want == "" {
				if !os.IsNotExist(err) {
					t.Errorf("%s holds %q (%v), want nothing", extPath, contents, err)
				}
				return
			}
			if err != nil || string(contents) != tt.want {
				t.Errorf("%s holds %q (%v), want %q", extPath, contents, err, tt.want)
			}
			if got := paths.IsSymlink(extPath); got != tt.wantLink {
				t.Errorf("%s is a symbolic link = %v, want %v", extPath, got, tt.wantLink)
			}
		})
	}
}

func TestUnlinkDirRestoresEveryFileInTheTree(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	files := map[string]string{
		".config/one":     "one\n",
		".config/sub/two": "two\n",
	}
	for rel, contents := range files {
		intPath := write(t, repoPath, filepath.Join("$HOME", rel), contents)
		extPath := filepath.Join(homeDir, rel)
		if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(intPath, extPath); err != nil {
			t.Fatal(err)
		}
	}

	if err := UnlinkDir(repoPath, filepath.Join(repoPath, repository.ContentDirName, "$HOME", ".config")); err != nil {
		t.Fatalf("UnlinkDir() = %v", err)
	}

	for rel, want := range files {
		extPath := filepath.Join(homeDir, rel)
		if paths.IsSymlink(extPath) {
			t.Errorf("%s is still a symbolic link", extPath)
		}
		if contents, err := os.ReadFile(extPath); err != nil || string(contents) != want {
			t.Errorf("%s holds %q (%v), want %q", extPath, contents, err, want)
		}
	}
}

// Unlink takes paths as they are named outside the repository, and hands each
// to UnlinkDir or UnlinkFile by what the repository holds at it
func TestUnlinkDispatchesOnWhatTheRepositoryHolds(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	for _, rel := range []string{".bashrc", ".config/conf"} {
		intPath := write(t, repoPath, filepath.Join("$HOME", rel), rel+"\n")
		extPath := filepath.Join(homeDir, rel)
		if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(intPath, extPath); err != nil {
			t.Fatal(err)
		}
	}

	err := Unlink(repoPath, []string{
		filepath.Join(homeDir, ".bashrc"),
		filepath.Join(homeDir, ".config"),
		filepath.Join(homeDir, ".vimrc"),
	})

	if err != nil {
		t.Fatalf("Unlink() = %v", err)
	}
	for _, rel := range []string{".bashrc", ".config/conf"} {
		if paths.IsSymlink(filepath.Join(homeDir, rel)) {
			t.Errorf("%s is still a symbolic link", rel)
		}
	}
}
