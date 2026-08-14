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

// ownNames are the entries a repository keeps for itself, which the legacy
// layout has to name because they sit among the paths it links. Only at a
// repository's top level: $HOME/.config/README.md is the operator's file.
var ownNames = map[string]bool{
	".git":       true,
	".github":    true,
	".gitignore": true,
	"LICENSE":    true,
	"README.md":  true,
}

// OwnName reports whether a top-level entry of a repository is one of the
// repository's own rather than a path to link.
func OwnName(name string) bool {
	return ownNames[name]
}

// ContentPath is the directory of repoPath whose tree is linked.
//
// A repository that has ContentDirName uses it. One that holds content at its
// top level is the layout gog wrote before this: it keeps working, and its own
// files are named above so that they are not linked. Move such a repository
// with `git mv`, and this follows.
//
// A repository with neither, which is every new one, takes ContentDirName, so
// that the first `gog add` writes the layout this version wants.
//
// Asked of the filesystem every time rather than answered once and kept: a
// repository whose layout is read while it is still empty would otherwise be
// remembered as new after its first file landed at its top level.
//
// CLEANUP (added 2026-08-14): once every repository holds ContentDirName, the
// legacy answer here, ownNames and link's guard for them can go, leaving the
// join below.
func ContentPath(repoPath string) string {
	if legacyLayout(repoPath) {
		return repoPath
	}
	return filepath.Join(repoPath, ContentDirName)
}

// legacyLayout reports whether repoPath keeps its content at its top level.
func legacyLayout(repoPath string) bool {
	if info, err := os.Stat(filepath.Join(repoPath, ContentDirName)); err == nil && info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		// Nothing can be read from it, and the two answers are not equally
		// wrong: a legacy repository read as new links nothing at all, while a
		// new one read as legacy has a top level holding only its own files,
		// every one of which is skipped. So the layout every existing
		// repository has is the answer to give.
		return true
	}
	return len(topLevelContent(entries)) > 0
}

// topLevelContent names the entries of a repository's top level that a legacy
// layout would link, in the order the directory gives them.
//
// Directories only. A repository's tree begins with a directory whichever path
// it names, $HOME or etc, while an unknown file there is a file the repository
// was given rather than one gog wrote: a .editorconfig or a Makefile that came
// with a template would otherwise be read as content and linked to the
// filesystem root, which is a stranger place for it than being left alone.
func topLevelContent(entries []os.DirEntry) []string {
	var out []string
	for _, entry := range entries {
		if entry.IsDir() && !OwnName(entry.Name()) && entry.Name() != ContentDirName {
			out = append(out, entry.Name())
		}
	}
	return out
}

// Stranded names what a repository holds beside the directory whose tree is
// linked: the part of a move that was not finished, which nothing links and
// nothing reports unless it is asked for.
//
// Empty for a repository of either layout, both of which keep their content in
// one place.
func Stranded(repoPath string) []string {
	if legacyLayout(repoPath) {
		return nil
	}
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil
	}
	return topLevelContent(entries)
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
