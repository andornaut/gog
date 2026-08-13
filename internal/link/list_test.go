package link

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
// holds
func TestListLeavesOutWhatIsNeverLinked(t *testing.T) {
	repoPath, homeDir := newSandbox(t)
	write(t, repoPath, "$HOME/.bashrc", "bashrc\n")
	write(t, repoPath, "$HOME/.cache/state", "state\n")
	for _, name := range []string{".gitignore", "LICENSE", "README.md"} {
		write(t, repoPath, name, name)
	}
	t.Setenv("GOG_IGNORE_FILES_REGEX", `\.cache/`)

	entries, err := List(repoPath)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	want := []string{filepath.Join(homeDir, ".bashrc")}
	if got := externalPaths(entries); !slices.Equal(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
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
	write(t, repoPath, "$HOME/.mine", "theirs\n")

	if err := os.Symlink(filepath.Join(repoPath, "$HOME", ".linked"), filepath.Join(homeDir, ".linked")); err != nil {
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
	if err := Dir(repoPath, repoPath); err == nil {
		t.Error("Dir() reported success although a conflict was listed")
	}
	for name, wantState := range want {
		extPath := filepath.Join(homeDir, name)
		target, readErr := os.Readlink(extPath)
		linked := readErr == nil && target == filepath.Join(repoPath, "$HOME", name)
		if linked != (wantState != StateConflict) {
			t.Errorf("%s linked = %v, but the listing said %s", name, linked, wantState)
		}
	}
}
