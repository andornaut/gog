package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// internalArgs precede every git command that gog runs on its own behalf, as
// opposed to one the user runs through `gog git`:
//
//   - --literal-pathspecs, because gog names files, and a name holding `*`, `?`
//     or `[` would otherwise match other files too.
//   - core.fsmonitor and core.hooksPath off, because `apply` links a
//     repository's files, which can include git's own configuration, and then
//     runs `git add`: a command named there would run before `apply` returns.
var internalArgs = []string{
	"--literal-pathspecs",
	"-c", "core.fsmonitor=false",
	"-c", "core.hooksPath=/dev/null",
}

// pathspecEnv lists the variables that change how git reads a pathspec, which
// gog's own commands always give literally. git refuses to combine some of them
// with --literal-pathspecs.
var pathspecEnv = []string{
	"GIT_LITERAL_PATHSPECS",
	"GIT_GLOB_PATHSPECS",
	"GIT_NOGLOB_PATHSPECS",
	"GIT_ICASE_PATHSPECS",
}

// ErrUnsafe reports a repository that git refuses to work in because another
// user owns it. The message is git's own, which names the safe.directory
// setting that allows it.
var ErrUnsafe = errors.New("git refuses to use this repository")

// Clone clones repoURL into repoPath. `--` ends the options, so that a URL
// beginning with a dash is not read as one. -q keeps git's progress off stdout,
// which carries only what a command produces.
func Clone(baseDir, repoPath string, repoURL string) error {
	return Run(baseDir, "clone", "-q", "--", repoURL, repoPath)
}

// Init initializes a git repository at repoPath. -q keeps git's confirmation off
// stdout, which carries only what a command produces.
func Init(baseDir, repoPath string) error {
	return Run(baseDir, "init", "-q", "--", repoPath)
}

// Is returns true if the given directory is the root of a git repository's
// work tree. The combined invocation prints "false" for a non-bare
// repository followed by the relative path to the top level, which is empty
// at the root itself. Bare repositories have no work tree to link from, so
// they are rejected.
//
// A repository that git refuses to use because another user owns it is
// reported as ErrUnsafe, with git's explanation, rather than as no repository:
// it is one, and the remedy is git's setting rather than removing it.
func Is(baseDir string) (bool, error) {
	cmd := internalCommand(baseDir, "rev-parse", "--is-bare-repository", "--show-cdup")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)) == "false", nil
	}
	if msg := stderr.String(); strings.Contains(msg, "safe.directory") {
		return false, fmt.Errorf("%w: %s", ErrUnsafe, strings.TrimSpace(msg))
	}
	return false, nil
}

// Output runs one of gog's own git commands in a repository and returns its
// standard output. Standard error stays attached to gog's own, so that git
// reports any failure itself.
func Output(baseDir string, arguments ...string) (string, error) {
	cmd := internalCommand(baseDir, arguments...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

// Run runs one of gog's own git commands in a repository, with gog's own
// streams attached
func Run(baseDir string, arguments ...string) error {
	cmd := internalCommand(baseDir, arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunCaptured runs one of gog's own git commands in a repository and returns
// what it wrote to standard error instead of passing it on, for a caller that
// reports the failure itself
func RunCaptured(baseDir string, arguments ...string) (string, error) {
	cmd := internalCommand(baseDir, arguments...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stderr.String()), err
}

// RunUser runs a git command the user gave through `gog git`, as they gave it,
// with gog's own streams attached
func RunUser(baseDir string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = baseDir
	cmd.Env = Env()
	return cmd.Run()
}

func internalCommand(baseDir string, arguments ...string) *exec.Cmd {
	cmd := exec.Command("git", append(slices.Clone(internalArgs), arguments...)...)
	cmd.Dir = baseDir
	cmd.Env = slices.DeleteFunc(Env(), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(pathspecEnv, name)
	})
	return cmd
}

// gitScrubbedEnv lists the environment variables that bind git to a specific
// repository, index, object store, pathspec prefix, or configuration source.
// It mirrors git's own repository-local and config-selection environment. gog
// runs git against the repository at cmd.Dir, so these are removed to avoid
// inheriting a location or configuration from an enclosing git invocation such
// as a git hook, which exports several of them.
var gitScrubbedEnv = map[string]bool{
	"GIT_DIR":                          true,
	"GIT_WORK_TREE":                    true,
	"GIT_INDEX_FILE":                   true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_COMMON_DIR":                   true,
	"GIT_NAMESPACE":                    true,
	"GIT_PREFIX":                       true,
	"GIT_SUPER_PREFIX":                 true,
	"GIT_GRAFT_FILE":                   true,
	"GIT_NO_REPLACE_OBJECTS":           true,
	"GIT_REPLACE_REF_BASE":             true,
	"GIT_SHALLOW_FILE":                 true,
	"GIT_CONFIG_GLOBAL":                true,
	"GIT_CONFIG_SYSTEM":                true,
	"GIT_CONFIG_NOSYSTEM":              true,
	"GIT_CONFIG_COUNT":                 true,
}

// gitScrubbedPrefixes lists the prefixes of the numbered configuration
// variables (GIT_CONFIG_KEY_<n> / GIT_CONFIG_VALUE_<n>) that inject config
// values into every git invocation. GIT_CONFIG_COUNT alone would disable them,
// but the key/value pairs are dropped as well so none can leak through.
var gitScrubbedPrefixes = []string{"GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"}

// Env returns the process environment without the variables that would
// redirect git away from the repository at cmd.Dir or override its configuration
func Env() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if shouldScrub(name) {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// shouldScrub reports whether an environment variable must be removed before
// running git, covering both the fixed names and the numbered config variables
func shouldScrub(name string) bool {
	if gitScrubbedEnv[name] {
		return true
	}
	for _, prefix := range gitScrubbedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
