package cmd

import (
	"github.com/andornaut/gog/cmd/repositorycmd"
	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/link"
	"github.com/andornaut/gog/internal/repository"
	"github.com/spf13/cobra"
)

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
		return git.Run(repoPath, args...)
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
	// Cannot add --repository as a persistent flag, because this breaks passthrough to `git`
	add.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	apply.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	remove.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	Cmd.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	Cmd.AddCommand(add, apply, git_, remove, repositorycmd.Cmd)
}
