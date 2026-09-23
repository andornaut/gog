// Package paths provides filesystem path helpers shared across gog.
package paths

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// Within returns true if p equals base or is contained within it. Matching on
// a path boundary prevents a sibling such as /home/alicebob from matching
// /home/alice. The trailing slash on base is trimmed so that the root
// directory "/" matches its contents.
//
// An empty base contains nothing: read as a prefix, it would match every
// absolute path, so a directory that was never resolved would appear to hold
// the whole filesystem.
func Within(base, p string) bool {
	if base == "" {
		return false
	}
	return p == base || strings.HasPrefix(p, strings.TrimSuffix(base, "/")+"/")
}

// Resolve returns p with its longest existing ancestor resolved through
// symlinks and the non-existent remainder appended, so that a path that does
// not yet exist can be compared against fully resolved paths.
//
// Only the existing prefix is canonical: the appended remainder is kept
// literally, so callers must ensure it does not yet exist as a symlink.
// A broken or looping symlink within the resolved prefix is also left
// literal rather than reported as an error.
func Resolve(p string) string {
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(Resolve(parent), filepath.Base(p))
}

// ResolveParent returns p with its parent directory resolved through symlinks
// and its last component kept literally, so that a symbolic link at p is named
// rather than followed
func ResolveParent(p string) string {
	p = filepath.Clean(p)
	return filepath.Join(Resolve(filepath.Dir(p)), filepath.Base(p))
}

// IsSymlink reports whether p is a symbolic link, and false if it cannot be
// examined at all
func IsSymlink(p string) bool {
	fileInfo, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink
}

// Display returns s with each character that is not printable written as its
// escape sequence, so that a file name holding a newline, a carriage return or
// a terminal control sequence cannot add lines to gog's output or rewrite what
// the terminal shows. Anything printable is left as it is.
func Display(s string) string {
	if strings.IndexFunc(s, isUnprintable) < 0 {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if !isUnprintable(r) {
			b.WriteRune(r)
			continue
		}
		quoted := strconv.QuoteRune(r)
		b.WriteString(quoted[1 : len(quoted)-1])
	}
	return b.String()
}

func isUnprintable(r rune) bool {
	return !unicode.IsPrint(r)
}
