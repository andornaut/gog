package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andornaut/gog/cmd/repositorycmd"
	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/link"
	"github.com/andornaut/gog/internal/repository"
)

// gitPathspecSubcommands lists the git subcommands whose every operand is a
// pathspec. Their arguments can be resolved without a separator; anywhere else
// an operand may be a revision, a branch, a remote, or the value of a flag.
var gitPathspecSubcommands = map[string]bool{
	"add":          true,
	"check-ignore": true,
	"clean":        true,
	"rm":           true,
	"stage":        true,
}

// resolveGitPaths converts symlinked paths to repo-relative paths so that git
// commands operate on the underlying files within the repository rather than on
// the symlinks outside it.
//
// Only the arguments git is certain to read as pathspecs are converted: those
// after a standalone `--`, and the operands of the subcommands listed above.
// Every other argument is passed through, because a name that happens to
// resolve to a managed path is usually not a path at all: `commit -m .bashrc`
// records the message `$HOME/.bashrc`, `branch wip` creates a branch named
// after the file, and the subcommand itself is rewritten into something git
// does not recognize.
func resolveGitPaths(repoPath string, args []string) []string {
	resolved := make([]string, len(args))
	copy(resolved, args)

	// Both sides of the comparison have to be free of symbolic links. A
	// repository can be reached through a symlinked parent (`/var` on macOS is
	// one), and a resolved argument measured against an unresolved repository
	// path looks like it lies outside the repository.
	realRepoPath, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		realRepoPath = repoPath
	}

	// The subcommand is the first argument only when no global flag precedes
	// it. Otherwise it is not identified at all and nothing before `--` is
	// converted, which errs towards passing arguments through unchanged.
	takesPathspecs := len(args) > 0 && gitPathspecSubcommands[args[0]]
	afterSeparator := false
	for i, arg := range args {
		switch {
		case afterSeparator:
			// A pathspec may begin with a dash once the separator has been given
		case arg == "--":
			afterSeparator = true
			continue
		case i == 0 || !takesPathspecs || strings.HasPrefix(arg, "-"):
			continue
		}
		resolved[i] = resolveGitPath(realRepoPath, arg)
	}
	return resolved
}

// resolveGitPath returns a pathspec relative to the repository if it resolves
// into the repository, and otherwise returns it unchanged. repoPath must
// already have its symbolic links resolved.
func resolveGitPath(repoPath, arg string) string {
	absPath, err := filepath.Abs(arg)
	if err != nil {
		return arg
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return arg
	}
	rel, err := filepath.Rel(repoPath, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return arg
	}
	return rel
}

// exitCodeError carries a git command's exit status so that `gog git` can exit
// with the status git itself returned, rather than collapsing every failure to
// 1. git has already reported the failure on its own stderr, so this message
// is never printed.
type exitCodeError struct {
	code int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("git exited with status %d", e.code)
}

// ExitCode returns the status that gog should exit with for the given error.
// Only `gog git` reports a status of its own; every other failure is gog's and
// exits 1.
func ExitCode(err error) int {
	var e *exitCodeError
	if errors.As(err, &e) {
		return e.code
	}
	return 1
}

var repositoryFlag string

var add = &cobra.Command{
	Use:                   "add [paths...]",
	Short:                 "Add files or directories to a repository",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		paths := cleanPaths(args)
		if err := repository.AddPaths(repoPath, paths); err != nil {
			return err
		}
		return link.Link(repoPath, paths)
	},
}

var apply = &cobra.Command{
	Use:                   "apply",
	Short:                 "Link a repository's contents to the filesystem",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		return link.Dir(repoPath, repoPath)
	},
}

var git_ = &cobra.Command{
	Use:                   "git [git command and arguments...]",
	Short:                 "Run a git command in a repository's directory",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	DisableSuggestions:    true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		err = git.Run(repoPath, resolveGitPaths(repoPath, args)...)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// git has already explained itself on stderr, so restating the
			// wait status here would only add noise above it
			c.SilenceErrors = true
			return &exitCodeError{code: exitErr.ExitCode()}
		}
		return err
	},
}

var remove = &cobra.Command{
	Use:                   "remove [paths...]",
	Short:                 "Remove files or directories from a repository",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}

		paths := cleanPaths(args)
		// Checked before anything is restored, so that an unusable path does
		// not leave the paths before it half-removed
		if err := repository.ValidateTargetPaths(paths); err != nil {
			return err
		}
		if err := link.Unlink(repoPath, paths); err != nil {
			return err
		}
		return repository.RemovePaths(repoPath, paths)
	},
}

// Cmd implements the root ./gog command
var Cmd = &cobra.Command{
	Use:              "gog [command]",
	Short:            "Link files to Git repositories",
	SilenceUsage:     true,
	TraverseChildren: true,
}

func init() {
	// -r is registered on each subcommand AND on the root (not as a persistent flag)
	// so that both `gog -r NAME <cmd>` and `gog <cmd> -r NAME` work. It is not
	// persistent because that would inherit it to `git`, which has its own -r flag
	// and uses DisableFlagParsing to pass arguments through.
	add.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	apply.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	remove.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	Cmd.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	Cmd.AddCommand(add, apply, git_, remove, repositorycmd.Cmd)
}
