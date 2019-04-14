package link

import (
	"fmt"
	"os"
	"strings"
)

func printError(p string, err interface{}) {
	fmt.Fprintf(os.Stderr, "ERROR %s %s\n", p, err)
}

func printLinked(intPath string, extPath string) {
	fmt.Printf("%s -> %s\n", extPath, escapeHomeVar(intPath))
}

func printUnLinked(intPath string, extPath string) {
	fmt.Printf("%s -> %s\n", escapeHomeVar(intPath), extPath)
}

func escapeHomeVar(p string) string {
	return strings.Replace(p, "$HOME", "\\$HOME", 1)
}
