package repositorycmd

import (
	"strings"
	"testing"
)

// Cobra's own message ("accepts between 1 and 2 arg(s), received 0") names
// neither the command nor what it wanted
func TestRequireArgs(t *testing.T) {
	want := "a repository name and an optional URL"
	check := requireArgs(1, 2, want)

	for _, args := range [][]string{{}, {"name", "url", "extra"}} {
		err := check(add, args)
		if err == nil || !strings.HasSuffix(err.Error(), "requires "+want) {
			t.Errorf("requireArgs(%q) = %v, want the command and what it wanted named", args, err)
		}
	}
	for _, args := range [][]string{{"name"}, {"name", "url"}} {
		if err := check(add, args); err != nil {
			t.Errorf("requireArgs(%q) = %v, want success", args, err)
		}
	}
}

// Naming a subcommand that does not exist is a wrong invocation, and so is
// naming none. Both are reported by the argument validator, which runs before
// usage is silenced, so the reader is shown the commands they could have named.
func TestNeedsCommand(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		// The command path is "repository" here and "gog repository" in the
		// binary, where the root has adopted it.
		{[]string{"bogus"}, `unknown command "bogus" for "repository"`},
		{nil, "repository requires a command"},
	} {
		if err := Cmd.Args(Cmd, tt.args); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Args(%q) = %v, want %q", tt.args, err, tt.want)
		}
	}
}
