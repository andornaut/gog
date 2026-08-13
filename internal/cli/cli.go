// Package cli holds what the command packages share.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// UsageError marks a wrong invocation: an unknown command, an unknown flag, or
// an operand a command does not take. gog exits 2 for these and 1 for a command
// that ran and failed, so that a script can tell them apart.
type UsageError struct{ err error }

func (e UsageError) Error() string { return e.err.Error() }

func (e UsageError) Unwrap() error { return e.err }

// Usage marks an existing error as a wrong invocation.
func Usage(err error) error { return UsageError{err} }

// Usagef reports a wrong invocation, as an argument validator does.
func Usagef(format string, a ...any) error { return UsageError{fmt.Errorf(format, a...)} }

// NeedsCommand refuses a command group invoked with no command, and an argument
// that names none. Both are wrong invocations: naming a command is how anything
// happens, and --help is how help is asked for. It validates arguments rather
// than running, so that the failure comes before usage is silenced and the
// reader is shown the commands they could have named.
func NeedsCommand(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return Usagef("unknown command %q for %q", args[0], c.CommandPath())
	}
	return Usagef("%s requires a command", c.CommandPath())
}
