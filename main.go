package main

import (
	"os"

	"github.com/andornaut/gog/cmd"
)

// version is the released version, set at build time by GoReleaser's default
// ldflags. A binary built any other way reports "dev".
var version = "dev"

// Execute starts the CLI
func main() {
	cmd.Cmd.Version = version
	if err := cmd.Cmd.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
