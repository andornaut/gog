package repositorycmd

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/andornaut/gog/internal/repository"
)

// Cmd implements ./gog repository
var Cmd = &cobra.Command{
	Use:          "repository [command]",
	Short:        "Manage repositories",
	SilenceUsage: true,
}

var isPath bool

var add = &cobra.Command{
	Use:                   "add [name] [url]",
	Short:                 "Add a git repository",
	Args:                  cobra.RangeArgs(1, 2),
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
		fmt.Printf("Added repository: %s\n", repoPath)
		return nil
	},
}

var getDefault = &cobra.Command{
	Use:                   "get-default [--path]",
	Short:                 "Print the name or path of the default repository",
	Long:                  "Either the first repository or the one defined by $GOG_DEFAULT_REPOSITORY_NAME",
	Args:                  cobra.NoArgs,
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

var list = &cobra.Command{
	Use:                   "list [--path]",
	Short:                 "Print the names or paths of all repositories",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		names, err := repository.List()
		if err != nil {
			return err
		}
		for _, msg := range names {
			if isPath {
				msg = path.Join(repository.BaseDir, msg)
			}
			fmt.Println(msg)
		}
		return nil
	},
}

var remove = &cobra.Command{
	Use:                   "remove [name]",
	Short:                 "Remove a repository",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(c *cobra.Command, args []string) error {
		repoName := args[0]
		repoPath, err := repository.Remove(repoName)
		if err != nil {
			return err
		}
		fmt.Printf("Removed repository: %s\n", repoPath)
		return nil
	},
}

func init() {
	getDefault.Flags().BoolVarP(&isPath, "path", "p", false, "print the path instead of the name")
	list.Flags().BoolVarP(&isPath, "path", "p", false, "print paths instead of names")
	Cmd.AddCommand(add, remove, getDefault, list)
}
