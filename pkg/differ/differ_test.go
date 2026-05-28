package differ

import (
	"testing"

	"github.com/agentroom/agentroom/pkg/index"
)

func mkIdx(files map[string]string) *index.Index {
	i := index.New()
	for p, sum := range files {
		i.Files[p] = index.Entry{SHA256: sum, Size: int64(len(sum)), Mode: "0644"}
	}
	return i
}

func TestDiff(t *testing.T) {
	from := mkIdx(map[string]string{
		"a.go": "aaa",
		"b.go": "bbb",
		"c.go": "ccc",
	})
	to := mkIdx(map[string]string{
		"a.go": "aaa",       // unchanged
		"b.go": "bbb-mod",   // modified
		"d.go": "ddd",       // added
	})
	cs := Diff(from, to)
	added, modified, deleted := PathsByOp(cs)
	if len(added) != 1 || added[0] != "d.go" {
		t.Errorf("added = %v", added)
	}
	if len(modified) != 1 || modified[0] != "b.go" {
		t.Errorf("modified = %v", modified)
	}
	if len(deleted) != 1 || deleted[0] != "c.go" {
		t.Errorf("deleted = %v", deleted)
	}
}

func TestConflicts(t *testing.T) {
	a := []Change{{Path: "x", Op: OpModified}, {Path: "y", Op: OpAdded}}
	b := []Change{{Path: "x", Op: OpModified}, {Path: "z", Op: OpDeleted}}
	got := Conflicts(a, b)
	if len(got) != 1 || got[0] != "x" {
		t.Errorf("conflicts = %v", got)
	}
}
