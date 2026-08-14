package repository

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/internal/paths"
)

// ContentDirName is the directory of a repository whose tree is linked. Every
// path under it names one on the filesystem; everything beside it belongs to
// the repository itself, so a README, a LICENSE or a forge's own .github
// directory cannot be mistaken for a file to link.
const ContentDirName = "root"

// ContentPath is the directory of repoPath whose tree is linked.
func ContentPath(repoPath string) string {
	return filepath.Join(repoPath, ContentDirName)
}

// HasContentDir reports whether the repository keeps its content where gog links
// from. A regular file or a link named ContentDirName is not that directory, and
// reading it as one would walk something that is not a tree.
func HasContentDir(repoPath string) bool {
	info, err := os.Stat(ContentPath(repoPath))
	return err == nil && info.IsDir()
}

// ToInternalPath converts an external path to one within the given repository
func ToInternalPath(repoPath, p string) string {
	if paths.Within(homeDir, p) {
		p = strings.TrimPrefix(p, homeDir)
		p = path.Join("$HOME", p)
	}
	return path.Join(ContentPath(repoPath), p)
}

// ToExternalPath converts an internal path to one outside of the given repository
func ToExternalPath(repoPath, p string) string {
	p = strings.TrimPrefix(p, ContentPath(repoPath)+"/")

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
