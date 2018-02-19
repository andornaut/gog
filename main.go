package main

import (
	"errors"
	"os"

	"github.com/andornaut/gog/cmd"
	"github.com/urfave/cli"
)

var pathFlag = cli.BoolFlag{
	Name:  "path, p",
	Usage: "Print the path instead of the name",
}
var repositoryFlag = cli.StringFlag{
	Name:  "repository, r",
	Usage: "`NAME` of the target repository",
}

func main() {
	app := cli.NewApp()
	app.Name = "gog"
	app.Usage = "Go Overlay Git"
	app.Description = "Link files to Git repositories"
	app.Version = "0.1.0"
	app.HideVersion = true
	app.HideHelp = true
	app.UsageText = "gog command [options] [arguments...]"
	app.Commands = []cli.Command{
		{
			Name:      "repository",
			Usage:     "Manage repositories",
			ArgsUsage: "[options] [arguments...]",
			HideHelp:  true,
			Subcommands: []cli.Command{
				{
					Name:      "add",
					Usage:     "Add and initialize a git repository",
					ArgsUsage: "<name> [url]",
					Action: func(c *cli.Context) error {
						repoName := c.Args().First()
						repoURL := c.Args().Get(1)
						if repoName == "" {
							return handleError(errors.New("Specify a repository name"))
						}
						return handleError(cmd.RunAddRepository(repoName, repoURL))
					},
				},
				{
					Name:      "remove",
					Usage:     "Remove a repository",
					ArgsUsage: "<name>",
					Action: func(c *cli.Context) error {
						repoName := c.Args().First()
						if repoName == "" {
							return handleError(errors.New("Specify a repository name"))
						}
						return handleError(cmd.RunRemoveRepository(repoName))
					},
				},
				{
					Name:      "get-default",
					Usage:     "Print the name of the default repository",
					UsageText: "gog repository get-default [--path]",
					Flags:     []cli.Flag{pathFlag},
					Action: func(c *cli.Context) error {
						isPath := c.Bool("path")
						return handleError(cmd.RunGetDefaultRepository(isPath))
					},
				},
				{
					Name:      "list",
					Usage:     "Print the names of all repositories",
					UsageText: "gog repository list [--path]",
					Flags:     []cli.Flag{pathFlag},
					Action: func(c *cli.Context) error {
						isPath := c.Bool("path")
						return handleError(cmd.RunListRepositories(isPath))
					},
				},
			},
		},
		{
			Name:      "add",
			Usage:     "Add files or directories to a repository",
			UsageText: "gog add [--repository NAME] <path> [paths...]",
			Flags:     []cli.Flag{repositoryFlag},
			Action: func(c *cli.Context) error {
				repoName := c.String("repository")
				paths := c.Args()
				if c.NArg() == 0 {
					return handleError(errors.New("Specify at least one file or directory path"))
				}
				return handleError(cmd.RunAdd(repoName, paths))
			},
		},
		{
			Name:      "remove",
			Usage:     "Remove files or directories from a repository",
			UsageText: "gog remove [--repository NAME] <path> [paths...]",
			Flags:     []cli.Flag{repositoryFlag},
			Action: func(c *cli.Context) error {
				repoName := c.String("repository")
				paths := c.Args()
				if c.NArg() == 0 {
					return handleError(errors.New("Specify at least one file or directory path"))
				}
				return handleError(cmd.RunRemove(repoName, paths))
			},
		},
		{
			Name:      "apply",
			Usage:     "Create symbolic links from a repository's files to the root filesystem",
			UsageText: "gog apply [--repository NAME]",
			Flags:     []cli.Flag{repositoryFlag},
			Action: func(c *cli.Context) error {
				repoName := c.String("repository")
				return handleError(cmd.RunApply(repoName))
			},
		},
		{
			Name:           "git",
			Usage:          "Run a git command in a repository",
			UsageText:      "gog git [--repository NAME] <git arguments> [git options...]",
			Flags:          []cli.Flag{repositoryFlag},
			SkipArgReorder: true,
			Action: func(c *cli.Context) error {
				repoName := c.String("repository")
				arguments := c.Args()
				return handleError(cmd.RunGit(repoName, arguments))
			},
		},
	}
	app.Run(os.Args)
}

func handleError(err error) error {
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}
