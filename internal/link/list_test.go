package link

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/andornaut/gog/internal/gittest"
	"github.com/andornaut/gog/internal/repository"
)

func externalPaths(entries []Entry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.ExternalPath)
	}
	return paths
}

func stateOf(t *testing.T, entries []Entry, extPath string) State {
	t.Helper()
	for _, entry := range entries {
		if entry.ExternalPath == extPath {
			return entry.State
		}
	}
	t.Fatalf("%s is not listed: %v", extPath, externalPaths(entries))
	return ""
}

// What is listed is what applying would act on, rather than what the directory
// holds, in the order a walk of the repository meets it
func TestListLeavesOutWhatIsNeverLinked(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	write(t, repoPath, "$HOME/.vimrc", "vimrc\n")
	// The repository's own files, which sit beside the directory whose tree is
	// linked rather than in it.
	for _, name := range []string{".gitignore", "LICENSE", "README.md"} {
		if err := os.WriteFile(filepath.Join(repoPath, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List(repoPath)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	want := []string{filepath.Join(homeDir, ".bashrc"), filepath.Join(homeDir, ".vimrc")}
	if got := externalPaths(entries); !slices.Equal(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

// A repository has no content directory until its first `gog add`, so listing
// one is not a failure
func TestListOnARepositoryWithNoContentDirectory(t *testing.T) {
	repoPath, _ := newSandbox(t)
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("readme"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := List(repoPath)

	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %v, want nothing", externalPaths(entries))
	}
}

// A path gog cannot examine is a conflict rather than a missing one: applying
// would report it and leave it alone rather than link it
func TestListStateOfAPathItCannotExamine(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory whose mode forbids it")
	}
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/locked/conf", "conf\n")
	locked := filepath.Join(homeDir, "locked")
	if err := os.Mkdir(locked, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0755) })

	entries, err := List(repoPath)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if got := stateOf(t, entries, filepath.Join(locked, "conf")); got != StateConflict {
		t.Errorf("state = %s, want %s", got, StateConflict)
	}
}

// The state has to be decided the way apply decides it, or a listing taken
// beforehand says the run will fail where it will succeed
func TestListStatesMatchWhatApplyingDoes(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.linked", "linked\n")
	write(t, repoPath, "$HOME/.missing", "missing\n")
	write(t, repoPath, "$HOME/.broken", "broken\n")
	write(t, repoPath, "$HOME/.same", "same\n")
	// The same length as what is in the way, so the contents decide
	write(t, repoPath, "$HOME/.mine", "them\n")

	if err := os.Symlink(filepath.Join(repoPath, repository.ContentDirName, "$HOME", ".linked"), filepath.Join(homeDir, ".linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(homeDir, "gone"), filepath.Join(homeDir, ".broken")); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{".same": "same\n", ".mine": "mine\n"} {
		if err := os.WriteFile(filepath.Join(homeDir, name), []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List(repoPath)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	want := map[string]State{
		".linked":  StateLinked,
		".missing": StateMissing,
		".broken":  StateReplace,
		".same":    StateReplace,
		".mine":    StateConflict,
	}
	for name, wantState := range want {
		extPath := filepath.Join(homeDir, name)
		if got := stateOf(t, entries, extPath); got != wantState {
			t.Errorf("state of %s = %s, want %s", name, got, wantState)
		}
	}

	// Everything the listing did not call a conflict is linked by the run that
	// follows, and the conflict is the only thing left alone
	if err := Dir(repoPath, false, repository.ContentPath(repoPath)); err == nil {
		t.Error("Dir() reported success although a conflict was listed")
	}
	for name, wantState := range want {
		extPath := filepath.Join(homeDir, name)
		target, readErr := os.Readlink(extPath)
		linked := readErr == nil && target == filepath.Join(repoPath, repository.ContentDirName, "$HOME", name)
		if linked != (wantState != StateConflict) {
			t.Errorf("%s linked = %v, but the listing said %s", name, linked, wantState)
		}
	}
}

// A path under a symbolically linked directory is listed as applying treats
// it: missing inside a link of the user's that applying descends through, and
// missing under a link of gog's that applying replaces with a real directory,
// whatever that link's own directory holds at the path
func TestListStatesUnderASymlinkedDirectory(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	userConf := write(t, repoPath, "$HOME/.config/app/conf", "conf\n")
	gogConf := write(t, repoPath, "$HOME/.local/app/conf", "conf\n")

	elsewhere := filepath.Join(homeDir, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(homeDir, ".config")); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(repository.BaseDir, "other", repository.ContentDirName, "$HOME", ".local")
	if err := os.MkdirAll(filepath.Join(other, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "app", "conf"), []byte("theirs\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(homeDir, ".local")); err != nil {
		t.Fatal(err)
	}

	entries, err := List(repoPath)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	for _, extPath := range []string{filepath.Join(homeDir, ".config/app/conf"), filepath.Join(homeDir, ".local/app/conf")} {
		if got := stateOf(t, entries, extPath); got != StateMissing {
			t.Errorf("state of %s = %s, want %s", extPath, got, StateMissing)
		}
	}

	if err := Dir(repoPath, false, repository.ContentPath(repoPath)); err != nil {
		t.Fatalf("Dir() = %v, although nothing was listed as a conflict", err)
	}
	assertLink(t, filepath.Join(elsewhere, "app/conf"), userConf)
	assertLink(t, filepath.Join(homeDir, ".local/app/conf"), gogConf)
}

// A link to a file the repository's history deleted, or that was deleted from
// its directory since, is stale. A link to a file it holds again is not, and
// neither is a path whose link is not this repository's.
func TestStaleListsLinksToFilesTheRepositoryNoLongerHolds(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	gone := write(t, repoPath, "$HOME/.gone", "gone\n")
	back := write(t, repoPath, "$HOME/.back", "back\n")
	uncommitted := write(t, repoPath, "$HOME/.uncommitted", "uncommitted\n")
	write(t, repoPath, "$HOME/.mine", "mine\n")
	gittest.Run(t, repoPath, "add", "-A")
	gittest.Run(t, repoPath, "commit", "-q", "-m", "init")
	for _, intPath := range []string{gone, back, uncommitted} {
		if err := os.Symlink(intPath, filepath.Join(homeDir, filepath.Base(intPath))); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".mine"), []byte("mine\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, repoPath, "rm", "-q", gone, back, filepath.Join(repository.ContentPath(repoPath), "$HOME", ".mine"))
	gittest.Run(t, repoPath, "commit", "-q", "-m", "remove")
	write(t, repoPath, "$HOME/.back", "back\n")
	if err := os.Remove(uncommitted); err != nil {
		t.Fatal(err)
	}

	stale, err := Stale(repoPath)
	if err != nil {
		t.Fatalf("Stale() = %v", err)
	}

	want := []string{filepath.Join(homeDir, ".gone"), filepath.Join(homeDir, ".uncommitted")}
	if !slices.Equal(stale, want) {
		t.Errorf("Stale() = %q, want %q", stale, want)
	}
}

// A repository with no commits has no history to consult
func TestStaleOnARepositoryWithNoCommits(t *testing.T) {
	repoPath, _ := newSandbox(t)

	if stale, err := Stale(repoPath); err != nil || len(stale) != 0 {
		t.Errorf("Stale() = %q, %v, want nothing", stale, err)
	}
}
