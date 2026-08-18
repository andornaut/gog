package main

import (
	"os"

	"github.com/andornaut/gog/cmd"
	"github.com/andornaut/gog/internal/version"
)

// Execute starts the CLI
func main() {
	cmd.Cmd.Version = version.Version
	if err := cmd.Cmd.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
