package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configureIn runs Configure against the given home directory, restoring what
// the package held afterwards
func configureIn(t *testing.T, home string, andThen ...func()) error {
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
	for _, set := range andThen {
		set()
	}
	return Configure()
}

func TestConfigureResolvesTheDataDirectory(t *testing.T) {
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

// $HOME is normalized so that path-boundary comparisons hold and so that paths
// under it are stored by the portable $HOME component rather than by name
func TestConfigureNormalizesTheHomeDirectory(t *testing.T) {
	home := t.TempDir()

	if err := configureIn(t, home+"/"); err != nil {
		t.Fatalf("Configure() = %v", err)
	}

	if homeDir != home {
		t.Errorf("homeDir = %q, want the trailing slash gone (%q)", homeDir, home)
	}
}

func TestConfigureRelativeHomeDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, "home"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := configureIn(t, "home"); err != nil {
		t.Fatalf("Configure() = %v", err)
	}

	if !filepath.IsAbs(homeDir) {
		t.Errorf("homeDir = %q, want an absolute path", homeDir)
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

func TestConfigureHonoursGogHome(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(t.TempDir(), "elsewhere")

	if err := configureIn(t, home, func() { t.Setenv("GOG_HOME", want) }); err != nil {
		t.Fatalf("Configure() = %v", err)
	}

	if BaseDir != want {
		t.Errorf("BaseDir = %q, want %q", BaseDir, want)
	}
}

func TestConfigureHonoursXdgDataHome(t *testing.T) {
	home := t.TempDir()
	dataHome := t.TempDir()

	if err := configureIn(t, home, func() { t.Setenv("XDG_DATA_HOME", dataHome) }); err != nil {
		t.Fatalf("Configure() = %v", err)
	}

	if want := filepath.Join(dataHome, "gog"); BaseDir != want {
		t.Errorf("BaseDir = %q, want %q", BaseDir, want)
	}
}
