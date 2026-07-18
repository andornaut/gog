/* MIT License
 *
 * Copyright (c) 2017 Roland Singer [roland.singer@desertbit.com]
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

// Package copy provides functions for copying files and directories.
// Originally derived from https://gist.github.com/r0l1/92462b38df26839a3ca324697c8cba04#file-copy-go-L118
package copy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// File copies the contents of the file named by src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all its contents will be replaced by the contents
// of the source file.
func File(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer func() {
		if e := in.Close(); e != nil && err == nil {
			err = e
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil && err == nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}

// SkipFunc is a callback that is used to determine whether or not to skip
// processing a directory entry
type SkipFunc func(string, string) bool

// isWithin returns true if p equals base or is contained within it,
// matching on a path boundary
func isWithin(base, p string) bool {
	return p == base || strings.HasPrefix(p, strings.TrimSuffix(base, "/")+"/")
}

// Dir recursively copies a directory tree. Source directory must exist.
// Symlinks are followed; a symlink that points to one of its own ancestor
// directories is a cycle and returns an error.
func Dir(src string, dst string, skipFunc SkipFunc) error {
	return copyDir(src, dst, filepath.Clean(dst), skipFunc, map[string]bool{})
}

// copyDir tracks the resolved paths of the directories on the current
// recursion path in `ancestors` in order to detect symlink cycles, and the
// top-level destination in `dstRoot` in order to detect sources that
// resolve into the tree being written
func copyDir(src, dst, dstRoot string, skipFunc SkipFunc, ancestors map[string]bool) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	resolvedSrc, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	if ancestors[resolvedSrc] {
		return fmt.Errorf("copy: symlink cycle detected at %s", src)
	}
	// A source that contains a directory already being copied is a cycle;
	// failing before descending avoids copying the ancestor's entire tree first
	for ancestor := range ancestors {
		if isWithin(resolvedSrc, ancestor) {
			return fmt.Errorf("copy: symlink cycle detected at %s (resolves to ancestor %s)", src, resolvedSrc)
		}
	}
	// A source inside the destination would be re-copied into itself endlessly
	if isWithin(dstRoot, resolvedSrc) {
		return fmt.Errorf("copy: source %s resolves inside the destination %s", src, dstRoot)
	}
	ancestors[resolvedSrc] = true
	defer delete(ancestors, resolvedSrc)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("copy: src must be a directory %s", src)
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Get file info for mode checking
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}

		if entryInfo.Mode()&os.ModeSymlink != 0 {
			srcPath, err = filepath.EvalSymlinks(srcPath)
			if err != nil {
				return err
			}
			// Re-stat the resolved target so that a symlink to a directory
			// is copied as a directory rather than opened as a file
			entryInfo, err = os.Stat(srcPath)
			if err != nil {
				return err
			}
		}
		if skipFunc(srcPath, dstPath) {
			continue
		}

		if entryInfo.IsDir() {
			err = copyDir(srcPath, dstPath, dstRoot, skipFunc, ancestors)
			if err != nil {
				return err
			}
		} else {
			err = File(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return
}
