package git

import (
	"os"
	"os/exec"
	"strings"
)

// Clone clones a git repository
func Clone(baseDir, repoPath string, repoURL string) error {
	return Run(baseDir, "clone", repoURL, repoPath)
}

// Init initializes a git repository
func Init(baseDir, repoPath string) error {
	return Run(baseDir, "init", repoPath)
}

// Is returns true if the given directory is the root of a git repository.
// `--show-cdup` prints the relative path to the top level, so empty output
// means the directory is the top level itself; this rejects subdirectories
// of an enclosing git repository without comparing paths.
func Is(baseDir string) bool {
	cmd := exec.Command("git", "rev-parse", "--show-cdup")
	cmd.Dir = baseDir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == ""
}

// Run runs a git command in a repository
func Run(baseDir string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = baseDir
	return cmd.Run()
}
