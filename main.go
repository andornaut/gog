package main

import (
	"os"

	"github.com/andornaut/gog/cmd"
)

// Execute starts the CLI
func main() {
	if err := cmd.Cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
