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

var (
	repositoryFlag string
	isStatus       bool
)

var add = &cobra.Command{
	Use:                   "add <paths>...",
	Short:                 "Add files or directories to a repository",
	Args:                  requirePaths,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		// The arguments are checked before the repository is selected, so that
		// an unusable one fails before anything is reported about a repository
		// the command will not use
		paths, err := cleanPaths(args)
		if err != nil {
			return err
		}
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
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
	Use:   "git [-r NAME] [git command and arguments...]",
	Short: "Run a git command in a repository's directory",
	Long: "Run a git command in a repository's directory, and exit with git's own\n" +
		"exit status. Every argument is handed to git, so `--help` reaches git\n" +
		"rather than gog: run `gog help git` for this text.\n\n" +
		"-r NAME selects the repository, and has to be the first argument for the\n" +
		"same reason: anywhere else it belongs to git.",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	DisableSuggestions:    true,
	RunE: func(c *cobra.Command, args []string) error {
		args, err := takeRepositoryFlag(args)
		if err != nil {
			return err
		}
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

var list = &cobra.Command{
	Use:   "list",
	Short: "Print the paths that a repository holds",
	Long: "Print the paths that `gog apply` would link, as they appear outside the\n" +
		"repository. The files a repository keeps for itself and whatever\n" +
		"GOG_IGNORE_FILES_REGEX names are left out.",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		entries, err := link.List(repoPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if isStatus {
				fmt.Printf("%-8s %s\n", entry.State, entry.ExternalPath)
				continue
			}
			fmt.Println(entry.ExternalPath)
		}
		return nil
	},
}

var remove = &cobra.Command{
	Use:                   "remove <paths>...",
	Short:                 "Remove files or directories from a repository",
	Args:                  requirePaths,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		paths, err := cleanPaths(args)
		if err != nil {
			return err
		}
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
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
	// The usage lines cobra derives from this list `gog [flags]` and
	// `gog [command]` separately, so naming the operand here would repeat one
	// of them
	Use:   "gog",
	Short: "Link files to Git repositories",
	// Runs once the arguments have been accepted and before any command does
	// its work, which is both where gog's environment has to be resolved and
	// where a failure stops being a wrong invocation worth printing usage for
	PersistentPreRunE: func(c *cobra.Command, args []string) error {
		c.SilenceUsage = true
		return repository.Configure()
	},
	// A command with nothing to run never has its arguments validated: cobra
	// prints help and reports success, so a mistyped command does nothing and
	// says nothing. Reporting it here is what makes an unknown command a failure.
	RunE: unknownCommand,
}

// takeRepositoryFlag consumes a leading -r/--repository option and returns the
// arguments that remain for git.
//
// `gog git` hands every argument to git, so the flag that selects the
// repository is read here rather than by cobra. Only the first argument is
// considered: git's own -r options belong to its subcommands (`git branch -r`),
// which cannot be the first argument, and one written anywhere else has to
// reach git untouched.
func takeRepositoryFlag(args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}
	switch arg := args[0]; {
	case arg == "-r", arg == "--repository":
		if len(args) < 2 {
			return nil, fmt.Errorf("flag needs an argument: %s", arg)
		}
		repositoryFlag = args[1]
		return args[2:], nil
	case strings.HasPrefix(arg, "--repository="):
		repositoryFlag = strings.TrimPrefix(arg, "--repository=")
		return args[1:], nil
	case strings.HasPrefix(arg, "-r") && len(arg) > 2:
		repositoryFlag = arg[2:]
		return args[1:], nil
	}
	return args, nil
}

// unknownCommand reports an argument that names no subcommand, and prints help
// when there is no argument at all
func unknownCommand(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown command %q for %q", args[0], c.CommandPath())
	}
	return c.Help()
}

// requirePaths validates the operands of the commands that take paths. Cobra's
// own message ("requires at least 1 arg(s), only received 0") names neither the
// command nor what it wanted.
func requirePaths(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s requires at least one path", c.CommandPath())
	}
	return nil
}

func init() {
	// -r belongs to the commands that select a repository, and is not persistent
	// because that would inherit it to `git`, which has its own -r flag and uses
	// DisableFlagParsing to pass arguments through. It is not registered on the
	// root either: `gog -r NAME repository list` would then be accepted and
	// ignored, since no `repository` subcommand selects a repository that way.
	for _, c := range []*cobra.Command{add, apply, list, remove} {
		c.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
		if err := c.RegisterFlagCompletionFunc("repository", repositorycmd.CompleteNames); err != nil {
			panic(err)
		}
	}
	list.Flags().BoolVarP(&isStatus, "status", "s", false, "print what applying would do to each path")
	// `git` parses its own arguments, so cobra's help flag would be listed
	// without being honored: --help reaches git like everything else
	git_.Flags().BoolP("help", "h", false, "")
	if err := git_.Flags().MarkHidden("help"); err != nil {
		panic(err)
	}
	// The generated completion command is noise in the listing of a program
	// with this few commands, and still works when it is not listed
	Cmd.CompletionOptions.HiddenDefaultCmd = true
	Cmd.AddCommand(add, apply, git_, list, remove, repositorycmd.Cmd)
}
