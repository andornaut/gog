package link

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/andornaut/gog/internal/git"
	"github.com/andornaut/gog/internal/repository"
)

// Stale returns the paths still linked to a file that the repository no longer
// holds: one its history deleted, such as by a pull, or one deleted from its
// directory since. Applying links only what the repository holds, so nothing
// else would name these links, and each one points at nothing.
func Stale(repoPath string) ([]string, error) {
	var deleted []string
	// A repository with no commits has no history to have deleted anything
	if _, err := git.Output(repoPath, "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		out, err := git.Output(repoPath, "log", "--no-renames", "--diff-filter=D", "--name-only", "--format=", "-z", "--", repository.ContentDirName)
		if err != nil {
			return nil, err
		}
		deleted = append(deleted, splitNul(out)...)
	}
	out, err := git.Output(repoPath, "ls-files", "--deleted", "-z", "--", repository.ContentDirName)
	if err != nil {
		return nil, err
	}
	deleted = append(deleted, splitNul(out)...)

	var stale []string
	for _, rel := range deleted {
		intPath := filepath.Join(repoPath, rel)
		extPath := repository.ToExternalPath(repoPath, intPath)
		if slices.Contains(stale, extPath) || !repository.LinksTo(extPath, intPath) {
			continue
		}
		// Linked to a path the repository holds again
		if _, err := filepath.EvalSymlinks(extPath); err == nil {
			continue
		}
		stale = append(stale, extPath)
	}
	slices.Sort(stale)
	return stale, nil
}

func splitNul(s string) []string {
	var fields []string
	for field := range strings.SplitSeq(s, "\x00") {
		if field = strings.TrimSpace(field); field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}
