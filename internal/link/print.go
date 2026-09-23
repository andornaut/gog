package link

import (
	"fmt"
	"os"
	"strings"

	"github.com/andornaut/gog/internal/paths"
)

// counts tallies what has been reported so far. linkFile prints and continues
// so that one unusable path does not stop the rest of the run, so these counts
// are what tell the caller that the run was incomplete. gog runs one command per
// process, so a package-level tally is enough.
type counts struct {
	// failures are paths that could not be linked or restored
	failures int
	// unstaged are paths that were linked but that git would not stage
	unstaged int
}

var reported counts

// printError reports a failure in the form every other gog failure takes. Every
// message names the path it concerns, so the path is not repeated here.
func printError(err error) {
	reported.failures++
	fmt.Fprintf(os.Stderr, "Error: %s\n", paths.Display(err.Error()))
}

// printUnstaged reports linked paths that git would not stage, followed by
// git's own explanation, which quotes any unusual name itself
func printUnstaged(extPaths []string, gitMsg string) {
	reported.unstaged += len(extPaths)
	if len(extPaths) == 1 {
		fmt.Fprintf(os.Stderr, "Error: linked %s, but git would not stage it:\n%s\n", paths.Display(extPaths[0]), gitMsg)
		return
	}
	fmt.Fprintf(os.Stderr, "Error: linked %d paths, but git would not stage them:\n%s\n", len(extPaths), gitMsg)
}

// Reported on stderr, with every other line gog writes about what it did.
// stdout carries what a caller consumes: `ls`, `repository ls`,
// `repository default` and git's own output.
func printLinked(intPath string, extPath string) {
	fmt.Fprintf(os.Stderr, "Linked: %s -> %s\n", paths.Display(extPath), paths.Display(escapeHomeVar(intPath)))
}

func printRestored(extPath string) {
	fmt.Fprintf(os.Stderr, "Restored: %s\n", paths.Display(escapeHomeVar(extPath)))
}

func escapeHomeVar(p string) string {
	return strings.Replace(p, "$HOME", "\\$HOME", 1)
}
