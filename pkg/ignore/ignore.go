package ignore

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Matcher decides whether a path is ignored (excluded from copy/sync).
type Matcher struct {
	gi *gitignore.GitIgnore
}

// Always-ignored entries (the agentroom internals themselves and VCS).
var hardExcludes = []string{
	".agent/",
	".git/",
	".agent-workspace.json",
}

// LoadFromFile loads .agentignore patterns from the given path. The file is optional.
func LoadFromFile(path string) (*Matcher, error) {
	var lines []string
	lines = append(lines, hardExcludes...)
	if data, err := os.ReadFile(path); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			lines = append(lines, l)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	gi := gitignore.CompileIgnoreLines(lines...)
	return &Matcher{gi: gi}, nil
}

// Match returns true if the given repo-relative path should be ignored.
// Pass isDir=true for directories so patterns like "foo/" can match.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	if relPath == "" || relPath == "." {
		return false
	}
	p := filepath.ToSlash(relPath)
	if isDir && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return m.gi.MatchesPath(p)
}
