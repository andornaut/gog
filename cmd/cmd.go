package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andornaut/gog/cmd/repositorycmd"
	"github.com/andornaut/gog/internal/cli"
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
// Every other argument is passed through, because a name that resolves to a
// managed path is usually not a path: `commit -m .bashrc` would record the
// message `$HOME/.bashrc`.
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
	// it. Otherwise it is not identified, and nothing before `--` is converted.
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

// exitCodeError carries a git command's exit status, so that `gog git` exits
// with the status git returned rather than collapsing every failure to 1. git
// has already reported the failure on its own stderr, so this message is never
// printed.
type exitCodeError struct {
	code int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("git exited with status %d", e.code)
}

// Exit codes. 2 is kept for a wrong invocation so that a script can tell a
// command it typed wrong from one that ran and failed.
const (
	exitFailed = 1
	exitUsage  = 2
)

// ExitCode returns the status that gog should exit with for the given error.
// Only `gog git` reports a status of its own.
func ExitCode(err error) int {
	if e, ok := errors.AsType[*exitCodeError](err); ok {
		return e.code
	}
	if _, ok := errors.AsType[cli.UsageError](err); ok {
		return exitUsage
	}
	return exitFailed
}

// Each command that selects a repository has its own flag variable, so that
// one command's run cannot read a name another command's flag left behind.
var (
	addRepositoryFlag   string
	applyRepositoryFlag string
	gitRepositoryFlag   string
	lsRepositoryFlag    string
	rmRepositoryFlag    string
	isForced            bool
	isStatus            bool
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
		repoPath, err := repoPath(addRepositoryFlag)
		if err != nil {
			return err
		}
		if err := repository.AddPaths(repoPath, isForced, paths); err != nil {
			return err
		}
		return link.Link(repoPath, paths)
	},
}

var apply = &cobra.Command{
	Use:                   "apply",
	Short:                 "Link a repository's contents to the filesystem",
	Args:                  noArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath(applyRepositoryFlag)
		if err != nil {
			return err
		}
		noteEmpty(repoPath)
		return link.Dir(repoPath, repository.ContentPath(repoPath))
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
		repoPath, err := repoPath(gitRepositoryFlag)
		if err != nil {
			return err
		}
		err = git.Run(repoPath, resolveGitPaths(repoPath, args)...)
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			// git has already explained itself on stderr, so restating the
			// wait status here would only add noise above it
			c.SilenceErrors = true
			return &exitCodeError{code: exitErr.ExitCode()}
		}
		return err
	},
}

var ls = &cobra.Command{
	Use:                   "ls",
	Short:                 "Print the paths that a repository holds",
	Long:                  "Print the paths that `gog apply` would link, as they appear outside the repository.",
	Args:                  noArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath(lsRepositoryFlag)
		if err != nil {
			return err
		}
		noteEmpty(repoPath)
		entries, err := link.List(repoPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if isStatus {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "%-8s %s\n", entry.State, entry.ExternalPath)
				continue
			}
			_, _ = fmt.Fprintln(c.OutOrStdout(), entry.ExternalPath)
		}
		return nil
	},
}

var rm = &cobra.Command{
	Use:                   "rm <paths>...",
	Short:                 "Remove files or directories from a repository",
	Args:                  requirePaths,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		paths, err := cleanPaths(args)
		if err != nil {
			return err
		}
		repoPath, err := repoPath(rmRepositoryFlag)
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
	// its work: where gog's environment is resolved, and where a failure stops
	// being a wrong invocation worth printing usage for
	PersistentPreRunE: func(c *cobra.Command, args []string) error {
		c.SilenceUsage = true
		return repository.Configure()
	},
	// A command with nothing to run never has its arguments validated: cobra
	// prints help and reports success. Validating here makes an unknown command
	// a failure, and replacing cobra's own check makes it exit 2.
	Args: cli.NeedsCommand,
	// Never reached, since the arguments never validate, but a command cobra
	// does not consider runnable has its arguments ignored altogether.
	RunE: func(c *cobra.Command, args []string) error { return nil },
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
		gitRepositoryFlag = args[1]
		return args[2:], nil
	case strings.HasPrefix(arg, "--repository="):
		gitRepositoryFlag = strings.TrimPrefix(arg, "--repository=")
		return args[1:], nil
	case strings.HasPrefix(arg, "-r") && len(arg) > 2:
		gitRepositoryFlag = arg[2:]
		return args[1:], nil
	}
	return args, nil
}

// noArgs refuses operands. cobra.NoArgs reports them as an unknown command,
// which misdescribes `gog apply /etc/hosts`: apply takes no operands at all,
// rather than one that was misspelled, and a wrong invocation exits 2.
func noArgs(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cli.Usagef("%s takes no arguments, but got %q", c.CommandPath(), args[0])
	}
	return nil
}

// requirePaths validates the operands of the commands that take paths. Cobra's
// own message ("requires at least 1 arg(s), only received 0") names neither the
// command nor what it wanted.
func requirePaths(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cli.Usagef("%s requires at least one path", c.CommandPath())
	}
	return nil
}

func init() {
	// -r belongs to the commands that select a repository, and is not persistent
	// because that would inherit it to `git`, which has its own -r flag and uses
	// DisableFlagParsing to pass arguments through. It is not registered on the
	// root either: `gog -r NAME repository ls` would then be accepted and
	// ignored, since no `repository` subcommand selects a repository that way.
	for _, selects := range []struct {
		c    *cobra.Command
		flag *string
	}{
		{add, &addRepositoryFlag},
		{apply, &applyRepositoryFlag},
		{ls, &lsRepositoryFlag},
		{rm, &rmRepositoryFlag},
	} {
		selects.c.Flags().StringVarP(selects.flag, "repository", "r", "", "name of repository")
		if err := selects.c.RegisterFlagCompletionFunc("repository", repositorycmd.CompleteNames); err != nil {
			panic(err)
		}
	}
	ls.Flags().BoolVarP(&isStatus, "status", "s", false, "print what applying would do to each path")
	add.Flags().BoolVar(&isForced, "force", false, "take a path over from the repository that manages it")
	// A flag cobra could not parse is a wrong invocation, and exits 2 like one.
	Cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return cli.Usage(err) })
	// -v is --vault in mrs, so --version is spelled out here rather than
	// answering to a letter that means something else in the tool beside it.
	Cmd.Flags().Bool("version", false, "version for gog")
	// `git` parses its own arguments, so cobra's help flag would be listed
	// without being honored: --help reaches git like everything else
	git_.Flags().BoolP("help", "h", false, "")
	if err := git_.Flags().MarkHidden("help"); err != nil {
		panic(err)
	}
	// The generated completion command still works when it is not listed, and
	// gog has too few commands to spend a line on it
	Cmd.CompletionOptions.HiddenDefaultCmd = true
	Cmd.AddCommand(add, apply, git_, ls, rm, repositorycmd.Cmd)
}
