package repositorycmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andornaut/gog/internal/cli"
	"github.com/andornaut/gog/internal/link"
	"github.com/andornaut/gog/internal/repository"
)

// Cmd implements ./gog repository
var Cmd = &cobra.Command{
	Use:   "repository",
	Short: "Manage repositories",
	// A command with nothing to run never has its arguments validated, so an
	// unknown subcommand would otherwise print help and report success
	Args: cli.NeedsCommand,
	RunE: func(c *cobra.Command, args []string) error { return nil },
}

// --path belongs to two commands, and each keeps its own variable so that
// neither can be handed the other's flag
var (
	isDefaultPath bool
	isLsPath      bool
	isForced      bool
)

// CompleteNames completes an argument or flag value that names a repository
func CompleteNames(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	names, err := repository.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var add = &cobra.Command{
	Use:                   "add <name> [url]",
	Short:                 "Add a git repository",
	Args:                  requireArgs(1, 2, "a repository name and an optional URL"),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoName := args[0]
		repoURL := ""
		if len(args) > 1 {
			repoURL = args[1]
		}

		repoPath, err := repository.Add(repoName, repoURL)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Added repository: %s\n", repoPath)
		return nil
	},
}

var getDefault = &cobra.Command{
	Use:                   "default [--path]",
	Short:                 "Print the name or path of the default repository",
	Long:                  "Either the first repository or the one defined by $GOG_DEFAULT_REPOSITORY_NAME",
	Args:                  noArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repository.GetDefault()
		if err != nil {
			return err
		}

		if isDefaultPath {
			_, _ = fmt.Fprintln(c.OutOrStdout(), repoPath)
		} else {
			_, _ = fmt.Fprintln(c.OutOrStdout(), filepath.Base(repoPath))
		}
		return nil
	},
}

var ls = &cobra.Command{
	Use:                   "ls [--path]",
	Short:                 "Print the names or paths of all repositories",
	Args:                  noArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		names, err := repository.List()
		if err != nil {
			return err
		}
		for _, msg := range names {
			if isLsPath {
				msg = filepath.Join(repository.BaseDir, msg)
			}
			_, _ = fmt.Fprintln(c.OutOrStdout(), msg)
		}
		return nil
	},
}

var rm = &cobra.Command{
	Use:                   "rm <name>",
	Short:                 "Remove a repository",
	Args:                  requireArgs(1, 1, "a repository name"),
	ValidArgsFunction:     CompleteNames,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repository.RemovalPath(args[0])
		if err != nil {
			return err
		}
		if !isForced {
			unsaved, unsavedErr := repository.UnsavedWork(repoPath)
			if unsavedErr != nil {
				return unsavedErr
			}
			if len(unsaved) > 0 {
				return fmt.Errorf("refusing to remove %s: it holds %s (pass --force to delete it anyway)",
					filepath.Base(repoPath), strings.Join(unsaved, " and "))
			}
		}
		// Restore what the repository holds before deleting it, so that the
		// user is left with their files rather than with links to nothing
		if err := link.UnlinkDir(repoPath, repository.ContentPath(repoPath)); err != nil {
			return err
		}
		if err := repository.Remove(repoPath); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Removed repository: %s\n", repoPath)
		return nil
	},
}

// noArgs refuses operands, reporting them as the wrong invocation they are
// rather than as an unknown command.
func noArgs(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cli.Usagef("%s takes no arguments, but got %q", c.CommandPath(), args[0])
	}
	return nil
}

// requireArgs validates an operand count. Cobra's own message ("accepts between
// 1 and 2 arg(s), received 0") names neither the command nor what it wanted.
func requireArgs(minArgs, maxArgs int, want string) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) < minArgs || len(args) > maxArgs {
			return cli.Usagef("%s requires %s", c.CommandPath(), want)
		}
		return nil
	}
}

func init() {
	// --path and --force are spelled out: -p is the password file and -f is
	// --full in mrs, and a letter that means two things across the tools is a
	// trap for the person typing, not for the parser.
	getDefault.Flags().BoolVar(&isDefaultPath, "path", false, "print the path instead of the name")
	ls.Flags().BoolVar(&isLsPath, "path", false, "print paths instead of names")
	rm.Flags().BoolVar(&isForced, "force", false, "remove even if the repository holds work that no remote has")
	Cmd.AddCommand(add, rm, getDefault, ls)
}
