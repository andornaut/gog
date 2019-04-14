package repository

import (
	"os"
	"path"
	"strings"
)

// ToInternalPath converts an external path to one within the given repository
func ToInternalPath(repoPath, p string) string {
	if strings.HasPrefix(p, homeDir) {
		p = strings.TrimPrefix(p, homeDir)
		p = path.Join("$HOME", p)
	}
	return path.Join(repoPath, p)
}

// ToExternalPath converts an internal path to one outside of the given repository
func ToExternalPath(repoPath, p string) string {
	p = os.ExpandEnv(strings.TrimPrefix(p, repoPath+"/"))
	// If p does not start with $HOME, then TrimPrefix will strip leading "/", so we must re-add it now.
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
