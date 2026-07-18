// Package paths provides filesystem path helpers shared across gog.
package paths

import (
	"path/filepath"
	"strings"
)

// Within returns true if p equals base or is contained within it. Matching on
// a path boundary prevents a sibling such as /home/alicebob from matching
// /home/alice. The trailing slash on base is trimmed so that the root
// directory "/" matches its contents.
func Within(base, p string) bool {
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
