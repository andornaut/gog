package main

import (
	"os"

	"github.com/andornaut/gog/cmd"
)

// version is the released version, stamped at link time by the ldflags in
// .goreleaser.yaml. A binary built any other way, the rolling dev archive
// included, reports "dev".
var version = "dev"

// Execute starts the CLI
func main() {
	cmd.Cmd.Version = version
	if err := cmd.Cmd.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
