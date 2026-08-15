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

// Package fscopy provides functions for copying files and directories. Named
// fscopy rather than copy so that importing it does not shadow the builtin.
// Originally derived from https://gist.github.com/r0l1/92462b38df26839a3ca324697c8cba04#file-copy-go-L118
package fscopy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/andornaut/gog/internal/paths"
)

// File copies the contents and mode of src to dst, creating dst or replacing
// what it holds.
func File(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if e := in.Close(); e != nil && err == nil {
			err = e
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if e := out.Close(); e != nil && err == nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	err = out.Sync()
	if err != nil {
		return err
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return err
	}

	return err
}

// SkipFunc reports whether a directory entry should be passed over
type SkipFunc func(string, string) bool

// ReportFunc is told about an entry that the copy left behind: a symbolic link
// or an irregular file. The caller decides what to say about it, so nothing is
// printed here.
type ReportFunc func(p string, mode os.FileMode)

// Dir recursively copies a directory tree. The source directory must exist.
//
// A symbolic link within the tree is reported and skipped rather than
// followed: copying a link's target would store the contents while discarding
// the link itself, and a broken link has no contents at all. Because no link is
// followed, the walk cannot meet a cycle or a path outside the tree.
//
// A destination directory is created only once there is something to put in it,
// so a source that is empty, or that holds nothing but skipped entries, leaves
// no trace at the destination.
func Dir(src string, dst string, skipFunc SkipFunc, report ReportFunc) error {
	// dst may not exist yet, so only its existing prefix can be resolved
	return copyDir(src, dst, paths.Resolve(dst), skipFunc, report, func() error {
		return os.MkdirAll(filepath.Dir(dst), 0755)
	})
}

// copyDir copies a single level of the tree. dstRoot is the resolved top-level
// destination, kept in order to reject a source inside the tree being written.
// ensureParent creates this directory's parent, and is called only once there
// is something to copy, so that a directory with nothing to hold is never
// created.
func copyDir(src, dst, dstRoot string, skipFunc SkipFunc, report ReportFunc, ensureParent func() error) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	resolvedSrc, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	// A source inside the destination would be re-copied into itself
	if paths.Within(dstRoot, resolvedSrc) {
		return fmt.Errorf("copy: source %q resolves inside the destination %q", src, dstRoot)
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("copy: src %q must be a directory", src)
	}

	// The entries are listed before anything is written, so a destination
	// nested inside this directory cannot be copied into itself
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	created := false
	ensureDst := func() error {
		if created {
			return nil
		}
		if err := ensureParent(); err != nil {
			return err
		}
		if err := os.Mkdir(dst, si.Mode()); err != nil && !os.IsExist(err) {
			return err
		}
		created = true
		return nil
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}

		if skipFunc(srcPath, dstPath) {
			continue
		}

		if entryInfo.IsDir() {
			if err := copyDir(srcPath, dstPath, dstRoot, skipFunc, report, ensureDst); err != nil {
				return err
			}
			continue
		}
		// A symbolic link, named pipe, socket or device node is left behind:
		// git stores neither its kind nor, for the irregular ones, its
		// contents, and opening one blocks until a writer appears or reads
		// without end. lstat reports a link as neither a directory nor a
		// regular file, so it is caught here, and no link is followed.
		if !entryInfo.Mode().IsRegular() {
			report(srcPath, entryInfo.Mode())
			continue
		}
		if err := ensureDst(); err != nil {
			return err
		}
		if err := File(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// FileKind names what a mode describes, for reporting a path that gog cannot
// manage. Only the kinds that can be met in a home directory are distinguished.
func FileKind(mode os.FileMode) string {
	switch {
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeNamedPipe != 0:
		return "named pipe"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode&os.ModeCharDevice != 0:
		return "character device"
	case mode&os.ModeDevice != 0:
		return "block device"
	case mode.IsRegular():
		return "file"
	}
	return "irregular file"
}
