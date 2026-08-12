package link

import (
	"fmt"
	"os"
	"strings"
)

// failures counts the errors reported so far. linkFile prints and continues so
// that one unusable path does not stop the rest of the run, so this count is
// what tells the caller that the run was incomplete. gog runs one command per
// process, so a package-level count is enough.
var failures int

func printError(p string, err error) {
	failures++
	fmt.Fprintf(os.Stderr, "ERROR %s %s\n", p, err)
}

func printLinked(intPath string, extPath string) {
	fmt.Printf("%s -> %s\n", extPath, escapeHomeVar(intPath))
}

func printRestored(extPath string) {
	fmt.Printf("Restored: %s\n", escapeHomeVar(extPath))
}

func escapeHomeVar(p string) string {
	return strings.Replace(p, "$HOME", "\\$HOME", 1)
}
