package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/andornaut/gog/git"
	"github.com/andornaut/gog/repository"
	"github.com/spf13/cobra"
)

var isPath bool

var repositoryCmd = &cobra.Command{
	Use:          "repository [command]",
	Short:        "Manage repositories",
	SilenceUsage: true,
}

var addRepositoryCmd = &cobra.Command{
	Use:   "add [name] [url]",
	Short: "Add a git repository",
	Args:  cobra.RangeArgs(1, 2),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoName := args[0]
		repoURL := ""
		if len(args) > 1 {
			repoURL = args[1]
		}

		if err := repository.ValidateRepoName(repoName); err != nil {
			return err
		}

		repoPath := path.Join(repository.BaseDir, repoName)
		if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
			return err
		}

		if repoURL == "" {
			if err := git.Init(repoPath); err != nil {
				return err
			}
		} else {
			if err := git.Clone(repoPath, repoURL); err != nil {
				return err
			}
		}
		fmt.Println(repoPath)
		return nil
	},
}

var removeRepositoryCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a repository",
	Args:  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoName := args[0]
		if err := repository.ValidateRepoName(repoName); err != nil {
			return err
		}
		repoPath := path.Join(repository.BaseDir, repoName)
		if err := os.RemoveAll(repoPath); err != nil {
			return err
		}
		fmt.Println(repoPath)
		return nil
	},
}

var getDefaultRepositoryCmd = &cobra.Command{
	Use:   "get-default [--path]",
	Short: "Print the name of the default repository",
	Args:  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoPath, err := repository.GetDefault()
		if err != nil {
			return err
		}

		if isPath {
			fmt.Println(repoPath)
		} else {
			fmt.Println(filepath.Base(repoPath))
		}
		return nil
	},
}

var listRepositoryCmd = &cobra.Command{
	Use:   "list [--path]",
	Short: "Print the names of all repositories",
	Args:  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		entries, err := ioutil.ReadDir(repository.BaseDir)
		if err != nil {
			return err
		}

		for _, fileInfo := range entries {
			if fileInfo.IsDir() {
				msg := fileInfo.Name()
				if isPath {
					msg = path.Join(repository.BaseDir, msg)
				}
				fmt.Println(msg)
			}
		}
		return nil
	},
}

func init() {
	getDefaultRepositoryCmd.Flags().BoolVarP(&isPath, "path", "p", false, "print the path of the default repository")
	listRepositoryCmd.Flags().BoolVarP(&isPath, "path", "p", false, "print the paths of all repositories")
	repositoryCmd.AddCommand(addRepositoryCmd, removeRepositoryCmd, getDefaultRepositoryCmd, listRepositoryCmd)
}
