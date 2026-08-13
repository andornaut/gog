package repository

import (
	"path"
	"strings"

	"github.com/andornaut/gog/internal/paths"
)

// ToInternalPath converts an external path to one within the given repository
func ToInternalPath(repoPath, p string) string {
	if paths.Within(homeDir, p) {
		p = strings.TrimPrefix(p, homeDir)
		p = path.Join("$HOME", p)
	}
	return path.Join(repoPath, p)
}

// ToExternalPath converts an internal path to one outside of the given repository
func ToExternalPath(repoPath, p string) string {
	p = strings.TrimPrefix(p, repoPath+"/")

	// Only $HOME is expanded, so that the environment cannot decide where a
	// repository writes, and only on a path boundary, so that a name such as
	// $HOMEWORK is left alone. Clean removes the double slash that a root home
	// directory would produce.
	if paths.Within("$HOME", p) {
		p = path.Clean(strings.Replace(p, "$HOME", homeDir, 1))
	}

	// A path that was not expanded lost its leading separator to TrimPrefix
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// SetHomeDirForTest sets homeDir and returns its previous value. Only tests
// use it.
func SetHomeDirForTest(dir string) string {
	original := homeDir
	homeDir = dir
	return original
}
