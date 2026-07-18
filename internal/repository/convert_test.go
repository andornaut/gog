package repository

import (
	"testing"
)

const (
	testHomeDir  = "/home/testuser"
	testRepoPath = testHomeDir + "/.local/share/gog/testrepo"
)

// TestToExternalPathRejectsArbitraryEnvVars tests critical security fix:
// ensures that only $HOME is expanded, not arbitrary environment variables
func TestToExternalPathRejectsArbitraryEnvVars(t *testing.T) {
	originalHomeDir := homeDir
	defer func() { homeDir = originalHomeDir }()

	homeDir = testHomeDir
	repoPath := testRepoPath

	tests := []struct {
		name     string
		p        string
		expected string
	}{
		{
			name:     "$HOME expansion works",
			p:        repoPath + "/$HOME/.bashrc",
			expected: "/home/testuser/.bashrc",
		},
		{
			name:     "$PATH not expanded (security)",
			p:        repoPath + "/$PATH/file",
			expected: "/$PATH/file",
		},
		{
			name:     "$USER not expanded (security)",
			p:        repoPath + "/$USER/.config",
			expected: "/$USER/.config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToExternalPath(repoPath, tt.p)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestToInternalPathMatchesHomeOnPathBoundary ensures that a sibling of the
// home directory (e.g. /home/testuserother) is not treated as being within it
func TestToInternalPathMatchesHomeOnPathBoundary(t *testing.T) {
	originalHomeDir := homeDir
	defer func() { homeDir = originalHomeDir }()

	homeDir = testHomeDir
	repoPath := testRepoPath

	tests := []struct {
		name     string
		p        string
		expected string
	}{
		{
			name:     "path within home is converted",
			p:        "/home/testuser/.bashrc",
			expected: repoPath + "/$HOME/.bashrc",
		},
		{
			name:     "sibling of home is not converted",
			p:        "/home/testuserother/.bashrc",
			expected: repoPath + "/home/testuserother/.bashrc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToInternalPath(repoPath, tt.p)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestToExternalPathMatchesHomeVarOnPathBoundary ensures that a path
// component that merely starts with $HOME (e.g. $HOMEWORK) is not expanded
func TestToExternalPathMatchesHomeVarOnPathBoundary(t *testing.T) {
	originalHomeDir := homeDir
	defer func() { homeDir = originalHomeDir }()

	homeDir = testHomeDir
	repoPath := testRepoPath

	result := ToExternalPath(repoPath, repoPath+"/$HOMEWORK/file")
	expected := "/$HOMEWORK/file"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

// TestPathConversionRootHome ensures a root home directory ("/") still maps
// paths under $HOME and round-trips without producing double slashes
func TestPathConversionRootHome(t *testing.T) {
	originalHomeDir := homeDir
	defer func() { homeDir = originalHomeDir }()

	homeDir = "/"
	repoPath := "/data/gog/testrepo"

	internal := ToInternalPath(repoPath, "/etc/foo")
	if internal != repoPath+"/$HOME/etc/foo" {
		t.Errorf("ToInternalPath() = %q, want %q", internal, repoPath+"/$HOME/etc/foo")
	}
	external := ToExternalPath(repoPath, internal)
	if external != "/etc/foo" {
		t.Errorf("ToExternalPath() = %q, want %q", external, "/etc/foo")
	}
}

// TestPathConversionRoundTrip verifies path conversion is reversible
func TestPathConversionRoundTrip(t *testing.T) {
	originalHomeDir := homeDir
	defer func() { homeDir = originalHomeDir }()

	homeDir = testHomeDir
	repoPath := testRepoPath

	testPaths := []string{
		"/home/testuser/.bashrc",
		"/home/testuser/.config/nvim/init.vim",
		"/etc/config",
	}

	for _, original := range testPaths {
		internal := ToInternalPath(repoPath, original)
		external := ToExternalPath(repoPath, internal)
		if external != original {
			t.Errorf("Round trip failed: %q -> %q -> %q", original, internal, external)
		}
	}
}
