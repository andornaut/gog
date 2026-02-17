package repository

import (
	"testing"
)

// TestToExternalPathRejectsArbitraryEnvVars tests critical security fix:
// ensures that only $HOME is expanded, not arbitrary environment variables
func TestToExternalPathRejectsArbitraryEnvVars(t *testing.T) {
	originalHomeDir := homeDir
	defer func() { homeDir = originalHomeDir }()

	homeDir = "/home/testuser"
	repoPath := "/home/testuser/.local/share/gog/testrepo"

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

// TestPathConversionRoundTrip verifies path conversion is reversible
func TestPathConversionRoundTrip(t *testing.T) {
	originalHomeDir := homeDir
	defer func() { homeDir = originalHomeDir }()

	homeDir = "/home/testuser"
	repoPath := "/home/testuser/.local/share/gog/testrepo"

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
