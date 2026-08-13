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

// Is returns true if the given directory is the root of a git repository's
// work tree. The combined invocation prints "false" for a non-bare
// repository followed by the relative path to the top level, which is empty
// at the root itself. Bare repositories have no work tree to link from, so
// they are rejected.
func Is(baseDir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-bare-repository", "--show-cdup")
	cmd.Dir = baseDir
	cmd.Env = commandEnv()
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "false"
}

// Output runs a git command in a repository and returns its standard output.
// Standard error stays attached to gog's own, so that git reports any failure
// itself.
func Output(baseDir string, arguments ...string) (string, error) {
	cmd := exec.Command("git", arguments...)
	cmd.Stderr = os.Stderr
	cmd.Dir = baseDir
	cmd.Env = commandEnv()
	out, err := cmd.Output()
	return string(out), err
}

// Run runs a git command in a repository
func Run(baseDir string, arguments ...string) error {
	cmd := exec.Command("git", arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = baseDir
	cmd.Env = commandEnv()
	return cmd.Run()
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

// commandEnv returns the process environment without the variables that would
// redirect git away from the repository at cmd.Dir or override its configuration
func commandEnv() []string {
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
