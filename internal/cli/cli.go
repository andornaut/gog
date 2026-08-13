// Package cli holds what the command packages share.
package cli

import "fmt"

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
