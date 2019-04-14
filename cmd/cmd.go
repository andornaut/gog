package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/andornaut/gog/cmd/repositorycmd"
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
		if err := repository.SyncRepository(repoPath, paths, repository.AddPath); err != nil {
			return err
		}
		return repository.SyncLinks(repoPath, paths, link.Dir, link.File)
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
		if err := repository.SyncLinks(repoPath, paths, link.UnlinkDir, link.UnlinkFile); err != nil {
			return err
		}
		return repository.SyncRepository(repoPath, paths, repository.RemovePath)
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

var git = &cobra.Command{
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
		return repository.GitRun(repoPath, args...)
	},
}

func cleanPaths(paths []string) []string {
	cleanedPaths := []string{}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		p, err := normalizePath(p)
		if err != nil {
			continue
		}
		cleanedPaths = append(cleanedPaths, p)
	}
	return cleanedPaths
}

func normalizePath(p string) (string, error) {
	if !path.IsAbs(p) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = path.Join(cwd, p)
	}

	return filepath.Clean(p), nil
}

func repoPath() (string, error) {
	repoPath, err := repository.RootPath(repositoryFlag)
	if err != nil {
		return "", err
	}
	fmt.Println("REPOSITORY:", filepath.Base(repoPath))
	return repoPath, nil
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
	add.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository to add to")
	apply.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository to apply")
	remove.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository to remove from")
	Cmd.Flags().StringVarP(&repositoryFlag, "repository", "r", "", "name of repository")
	Cmd.AddCommand(add, apply, git, remove, repositorycmd.Cmd)
}
