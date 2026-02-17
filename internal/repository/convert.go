package repository

import (
	"path"
	"strings"
)

// ToInternalPath converts an external path to one within the given repository
func ToInternalPath(repoPath, p string) string {
	if strings.HasPrefix(p, homeDir) {
		p = strings.TrimPrefix(p, homeDir)
		p = path.Join("$HOME", p)
	}
	return path.Join(repoPath, p)
}

// ToExternalPath converts an internal path to one outside of the given repository
func ToExternalPath(repoPath, p string) string {
	p = strings.TrimPrefix(p, repoPath+"/")

	// Only expand $HOME specifically, not arbitrary environment variables
	// This prevents path injection attacks via malicious environment variables
	if strings.HasPrefix(p, "$HOME") {
		p = strings.Replace(p, "$HOME", homeDir, 1)
	}

	// If p does not start with $HOME and was expanded, then TrimPrefix stripped leading "/", so we must re-add it now.
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// SetHomeDirForTest sets homeDir for testing and returns the original value.
// This should only be used in tests to mock the home directory.
func SetHomeDirForTest(dir string) string {
	original := homeDir
	homeDir = dir
	return original
}
