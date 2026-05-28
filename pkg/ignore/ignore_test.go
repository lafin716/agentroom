package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatch(t *testing.T) {
	dir := t.TempDir()
	ignoreFile := filepath.Join(dir, ".agentignore")
	if err := os.WriteFile(ignoreFile, []byte(".env\n*.key\nsecrets/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadFromFile(ignoreFile)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{".env", false, true},
		{"src/foo.go", false, false},
		{"key.pem", false, false},
		{"server.key", false, true},
		{"secrets", true, true},
		{".git", true, true},
		{".agent", true, true},
		{"normal/file.txt", false, false},
	}
	for _, c := range cases {
		if got := m.Match(c.path, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestMatchNoIgnoreFile(t *testing.T) {
	dir := t.TempDir()
	m, err := LoadFromFile(filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match(".git", true) {
		t.Errorf(".git/ should always be ignored")
	}
	if m.Match("foo.go", false) {
		t.Errorf("foo.go should not be ignored without rules")
	}
}
