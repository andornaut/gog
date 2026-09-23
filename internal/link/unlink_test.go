package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andornaut/gog/internal/paths"
	"github.com/andornaut/gog/internal/repository"
	"github.com/andornaut/gog/internal/testout"
)

func TestUnlinkFileRestoresWhatTheLinkPointedAt(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	extPath := filepath.Join(homeDir, ".bashrc")
	if err := os.Symlink(intPath, extPath); err != nil {
		t.Fatal(err)
	}

	out := testout.Capture(t, func() {
		if err := UnlinkFile(repoPath, intPath); err != nil {
			t.Errorf("UnlinkFile() = %v", err)
		}
	})

	if paths.IsSymlink(extPath) {
		t.Errorf("%s is still a symbolic link", extPath)
	}
	if contents, err := os.ReadFile(extPath); err != nil || string(contents) != "bashrc\n" {
		t.Errorf("%s holds %q (%v), want the repository's contents", extPath, contents, err)
	}
	// The result line names the path that was given back
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
				t.Helper()
				if err := os.WriteFile(extPath, []byte("mine\n"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			want: "mine\n",
		},
		{
			name: "a link to another repository's copy",
			prepare: func(t *testing.T, extPath string) {
				t.Helper()
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

// A repository has no content directory until its first `gog add`, so there is
// nothing to restore rather than a tree that could not be walked. `gog
// repository rm` restores what a repository holds before deleting it.
func TestUnlinkDirOnARepositoryWithNoContentDirectory(t *testing.T) {
	repoPath, _ := newSandbox(t)

	if err := UnlinkDir(repoPath, repository.ContentPath(repoPath)); err != nil {
		t.Errorf("UnlinkDir() = %v", err)
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

// `gog rm` names a path, and the repository's copy is removed next, so a path
// whose link was deleted is given the file back rather than losing it
func TestUnlinkRestoresANamedPathWithNothingAtIt(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.config/app/conf", "conf\n")
	extPath := filepath.Join(homeDir, ".config", "app", "conf")

	testout.Capture(t, func() {
		if err := Unlink(repoPath, []string{extPath}); err != nil {
			t.Errorf("Unlink() = %v", err)
		}
	})

	if contents, err := os.ReadFile(extPath); err != nil || string(contents) != "conf\n" {
		t.Errorf("%s holds %q (%v), want the repository's contents", extPath, contents, err)
	}
}

// A path that cannot be examined may hold the link, so it fails rather than
// being taken for one with nothing of gog's at it
func TestUnlinkFileFailsOnAPathItCannotExamine(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory whatever its mode")
	}
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/priv/f", "f\n")
	priv := filepath.Join(homeDir, "priv")
	if err := os.MkdirAll(priv, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(intPath, filepath.Join(priv, "f")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(priv, 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(priv, 0755) })

	if err := UnlinkFile(repoPath, intPath); err == nil {
		t.Error("UnlinkFile() reported success for a path it could not examine")
	}
}

// A symbolic link that the repository holds is restored as that link. Its
// target is never followed: it may be a file of the user's, and a clone of
// someone else's repository chooses it.
func TestUnlinkFileRestoresALinkTheRepositoryHolds(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	mine := filepath.Join(homeDir, ".profile")
	if err := os.WriteFile(mine, []byte("mine\n"), 0644); err != nil {
		t.Fatal(err)
	}
	intPath := filepath.Join(repository.ContentPath(repoPath), "$HOME", ".alias")
	if err := os.MkdirAll(filepath.Dir(intPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(mine, intPath); err != nil {
		t.Fatal(err)
	}
	// The path the committed link names, and one gog linked to it
	profileInt := filepath.Join(repository.ContentPath(repoPath), "$HOME", ".profile")
	if err := os.Symlink(mine, profileInt); err != nil {
		t.Fatal(err)
	}
	aliasExt := filepath.Join(homeDir, ".alias")
	if err := os.Symlink(intPath, aliasExt); err != nil {
		t.Fatal(err)
	}

	testout.Capture(t, func() {
		if err := UnlinkDir(repoPath, repository.ContentPath(repoPath)); err != nil {
			t.Errorf("UnlinkDir() = %v", err)
		}
	})

	if contents, err := os.ReadFile(mine); err != nil || string(contents) != "mine\n" {
		t.Errorf("%s holds %q (%v), want it left alone", mine, contents, err)
	}
	if paths.IsSymlink(mine) {
		t.Errorf("%s was replaced by a link", mine)
	}
	if target, err := os.Readlink(aliasExt); err != nil || target != mine {
		t.Errorf("%s -> %s (%v), want the repository's link to %s", aliasExt, target, err, mine)
	}
}

// A path reached through a symbolic link into gog's data directory is the
// repository's copy itself, which the caller is about to remove
func TestUnlinkFileRefusesAPathReachedThroughTheDataDirectory(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	intPath := write(t, repoPath, "$HOME/.config/app/conf", "conf\n")
	if err := os.MkdirAll(filepath.Join(homeDir, ".config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(intPath), filepath.Join(homeDir, ".config", "app")); err != nil {
		t.Fatal(err)
	}

	if err := UnlinkFile(repoPath, intPath); err == nil {
		t.Error("UnlinkFile() reported success for the repository's own copy")
	}
	if contents, err := os.ReadFile(intPath); err != nil || string(contents) != "conf\n" {
		t.Errorf("%s holds %q (%v), want it kept", intPath, contents, err)
	}
}
