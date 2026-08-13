package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configureIn runs Configure against the given home directory, restoring what
// the package held afterwards
func configureIn(t *testing.T, home string) error {
	t.Helper()
	originalBase, originalHome := BaseDir, homeDir
	t.Cleanup(func() {
		BaseDir = originalBase
		homeDir = originalHome
	})
	t.Setenv("HOME", home)
	// Whatever the developer has set would otherwise decide where the data
	// directory lands, and Configure creates it
	t.Setenv("GOG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	return Configure()
}

func TestConfigureResolvesAndCreatesTheDataDirectory(t *testing.T) {
	home := t.TempDir()

	if err := configureIn(t, home); err != nil {
		t.Fatalf("Configure() = %v", err)
	}

	want := filepath.Join(home, ".local/share/gog")
	if BaseDir != want {
		t.Errorf("BaseDir = %q, want %q", BaseDir, want)
	}
	if info, err := os.Stat(BaseDir); err != nil || !info.IsDir() {
		t.Errorf("the data directory was not created: %v", err)
	}
	if homeDir != home {
		t.Errorf("homeDir = %q, want %q", homeDir, home)
	}
}

// $HOME is normalized so that path-boundary comparisons hold, and so that paths
// under it are stored by the portable $HOME component rather than by name
func TestConfigureNormalizesTheHomeDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(cwd, "home"), 0755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cwd, "home")

	tests := []struct {
		name string
		home string
	}{
		{name: "a trailing slash is dropped", home: want + "/"},
		{name: "a relative path is resolved", home: "home"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := configureIn(t, tt.home); err != nil {
				t.Fatalf("Configure() = %v", err)
			}
			if homeDir != want {
				t.Errorf("homeDir = %q, want %q", homeDir, want)
			}
		})
	}
}

// A home directory that cannot be used is a misconfigured environment rather
// than a new one: without this, gog creates its data directory under whatever
// $HOME happens to name and reports success
func TestConfigureRefusesAnUnusableHomeDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		home string
		want string
	}{
		{name: "one that does not exist", home: filepath.Join(root, "gone"), want: "home directory does not exist"},
		{name: "one that is a file", home: file, want: "home directory is not a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := configureIn(t, tt.home)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Configure() = %v, want %q", err, tt.want)
			}
		})
	}
}

// The data directory is chosen from the environment and normalized: a trailing
// slash or a relative path would break the path-boundary comparisons that
// decide what gog owns
func TestGetBaseDir(t *testing.T) {
	tests := []struct {
		name     string
		gogHome  string
		dataHome string
		want     string
	}{
		{name: "the default", want: "/home/testuser/.local/share/gog"},
		{name: "XDG_DATA_HOME", dataHome: "/data", want: "/data/gog"},
		{name: "GOG_HOME wins over XDG_DATA_HOME", gogHome: "/elsewhere", dataHome: "/data", want: "/elsewhere"},
		{name: "a trailing slash is dropped", gogHome: "/data/gog/", want: "/data/gog"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOG_HOME", tt.gogHome)
			t.Setenv("XDG_DATA_HOME", tt.dataHome)

			got, err := getBaseDir("/home/testuser")

			if err != nil {
				t.Fatalf("getBaseDir() = %v", err)
			}
			if got != tt.want {
				t.Errorf("getBaseDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBaseDirResolvesARelativeGogHome(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOG_HOME", "relative-gog")
	t.Setenv("XDG_DATA_HOME", "")

	got, err := getBaseDir("/home/testuser")

	if err != nil {
		t.Fatalf("getBaseDir() = %v", err)
	}
	if want := filepath.Join(cwd, "relative-gog"); got != want {
		t.Errorf("getBaseDir() = %q, want %q", got, want)
	}
}
