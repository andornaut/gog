package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andornaut/gog/git"
	"github.com/andornaut/gog/link"
	"github.com/andornaut/gog/repository"
	"github.com/spf13/cobra"
)

// Execute starts the CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var repositoryFlag string

var rootCmd = &cobra.Command{
	Use:          "gog [command]",
	Short:        "Link files to Git repositories",
	SilenceUsage: true,
}

var addCmd = &cobra.Command{
	Use:   "add [paths...]",
	Short: "Add files or directories to a repository",
	Args:  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		return runAdd(repoPath, args)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove [paths...]",
	Short: "Remove files or directories from a repository",
	Args:  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		return runRemove(repoPath, args)
	},
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create symbolic links from a repository's files to the root filesystem",
	Args:  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		return link.Dir(repoPath, repoPath)
	},
}

var gitCmd = &cobra.Command{
	Use:   "git [git arguments...]",
	Short: "Run a git command in a repository",
	DisableFlagsInUseLine: true,
	DisableSuggestions:    true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repoPath()
		if err != nil {
			return err
		}
		return git.Run(repoPath, args)
	},
}

func repoPath() (string, error) {
	repoPath, err := repository.RootPath(repositoryFlag)
	if err != nil {
		return "", err
	}
	fmt.Println("REPOSITORY:", filepath.Base(repoPath))
	return repoPath, nil
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	rootCmd.AddCommand(addCmd, applyCmd, gitCmd, removeCmd, repositoryCmd)
}
