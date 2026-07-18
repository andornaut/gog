package repository

import (
	"path"
	"strings"
)

// hasPathPrefix returns true if p equals base or is contained within it.
// Matching on a path boundary prevents a sibling such as /home/alicebob from
// matching /home/alice. The trailing slash on base is trimmed so that the
// root directory "/" matches its contents.
func hasPathPrefix(base, p string) bool {
	return p == base || strings.HasPrefix(p, strings.TrimSuffix(base, "/")+"/")
}

// ToInternalPath converts an external path to one within the given repository
func ToInternalPath(repoPath, p string) string {
	if hasPathPrefix(homeDir, p) {
		p = strings.TrimPrefix(p, homeDir)
		p = path.Join("$HOME", p)
	}
	return path.Join(repoPath, p)
}

// ToExternalPath converts an internal path to one outside of the given repository
func ToExternalPath(repoPath, p string) string {
	p = strings.TrimPrefix(p, repoPath+"/")

	// Only expand $HOME specifically, not arbitrary environment variables
	// This prevents path injection attacks via malicious environment variables.
	// Match on a path boundary so that a file named e.g. $HOMEWORK is not expanded.
	// Clean removes the double slash that a root home directory would produce.
	if hasPathPrefix("$HOME", p) {
		p = path.Clean(strings.Replace(p, "$HOME", homeDir, 1))
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
