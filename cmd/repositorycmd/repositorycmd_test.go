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

// Naming a subcommand that does not exist is a wrong invocation, so the usage
// that the root silences for running commands is restored
func TestUnknownSubcommandRestoresUsage(t *testing.T) {
	Cmd.SilenceUsage = true
	t.Cleanup(func() { Cmd.SilenceUsage = false })

	err := Cmd.RunE(Cmd, []string{"bogus"})

	if err == nil || !strings.Contains(err.Error(), `unknown command "bogus"`) {
		t.Errorf("RunE() = %v, want the command named", err)
	}
	if Cmd.SilenceUsage {
		t.Error("usage is still silenced for a wrong invocation")
	}
}
