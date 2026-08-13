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

	"github.com/andornaut/gog/internal/paths"
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

// ReportFunc is told about an entry that the copy left behind, and is called
// for a symbolic link and for an irregular file. What such an entry means to
// the caller is the caller's to say, so nothing is printed here.
type ReportFunc func(p string, mode os.FileMode)

// Dir recursively copies a directory tree. The source directory must exist.
//
// A symbolic link within the tree is reported and skipped rather than
// followed. Copying a link's target would store the contents while discarding
// the link itself, and a broken link has no contents to store at all. Because
// no link is followed, the tree cannot contain a cycle or reach outside
// itself.
//
// A destination directory is created only once there is something to put in
// it, so a source directory that is empty - or that holds nothing but skipped
// entries - leaves no trace at the destination.
func Dir(src string, dst string, skipFunc SkipFunc, report ReportFunc) error {
	// dst may not exist yet, so only its existing prefix can be resolved
	return copyDir(src, dst, paths.Resolve(dst), skipFunc, report, func() error {
		return os.MkdirAll(filepath.Dir(dst), 0755)
	})
}

// copyDir copies a single level of the tree. `dstRoot` is the resolved
// top-level destination, retained in order to reject a source that lives
// inside the tree being written. `ensureParent` creates this directory's
// parent and is only called once there is something to copy, so that a
// directory with nothing to hold is never created.
func copyDir(src, dst, dstRoot string, skipFunc SkipFunc, report ReportFunc, ensureParent func() error) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	resolvedSrc, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	// A source inside the destination would be re-copied into itself
	if paths.Within(dstRoot, resolvedSrc) {
		return fmt.Errorf("copy: source %s resolves inside the destination %s", src, dstRoot)
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("copy: src must be a directory %s", src)
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
		// A symbolic link, named pipe, socket or device node is not something a
		// copy can carry: git stores neither the kind nor, for the irregular
		// ones, the contents, and opening one to read it blocks until a writer
		// appears or reads without end. A link is caught here rather than by its
		// own test because lstat reports it as neither a directory nor a regular
		// file, which is also why no link is followed and the walk can meet
		// neither a cycle nor a path outside the tree.
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
