// Package testout captures what gog writes to standard error in tests.
package testout

import (
	"os"
	"testing"
)

// Capture returns what f writes to standard error. gog reports what it did on
// the process's own stream rather than through a writer the caller supplies, so
// a test that checks a message has to take it from the process.
//
// A file rather than a pipe: git inherits whatever os.Stderr is when gog runs
// it, and a pipe is only read to its end once every process holding the write
// end has let go of it.
func Capture(t *testing.T, f func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = file
	defer func() { os.Stderr = original }()

	f()
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
