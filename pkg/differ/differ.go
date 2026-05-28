package differ

import (
	"sort"

	"github.com/agentroom/agentroom/pkg/index"
)

type Op string

const (
	OpAdded    Op = "added"
	OpModified Op = "modified"
	OpDeleted  Op = "deleted"
)

// Change is a single difference between two indexes.
type Change struct {
	Path string
	Op   Op
	// Old is the entry before; nil for added.
	Old *index.Entry
	// New is the entry after; nil for deleted.
	New *index.Entry
}

// Diff returns the changes that turn "from" into "to". Both indexes must use the same root.
func Diff(from, to *index.Index) []Change {
	var changes []Change
	if from == nil {
		from = index.New()
	}
	if to == nil {
		to = index.New()
	}
	seen := map[string]struct{}{}
	for p, fEntry := range from.Files {
		seen[p] = struct{}{}
		tEntry, ok := to.Files[p]
		if !ok {
			fc := fEntry
			changes = append(changes, Change{Path: p, Op: OpDeleted, Old: &fc})
			continue
		}
		if fEntry.SHA256 != tEntry.SHA256 || fEntry.Size != tEntry.Size {
			fc, tc := fEntry, tEntry
			changes = append(changes, Change{Path: p, Op: OpModified, Old: &fc, New: &tc})
		}
	}
	for p, tEntry := range to.Files {
		if _, ok := seen[p]; ok {
			continue
		}
		tc := tEntry
		changes = append(changes, Change{Path: p, Op: OpAdded, New: &tc})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

// PathsByOp groups change paths by their operation.
func PathsByOp(cs []Change) (added, modified, deleted []string) {
	for _, c := range cs {
		switch c.Op {
		case OpAdded:
			added = append(added, c.Path)
		case OpModified:
			modified = append(modified, c.Path)
		case OpDeleted:
			deleted = append(deleted, c.Path)
		}
	}
	return
}

// Conflicts returns paths that appear in both change sets.
func Conflicts(a, b []Change) []string {
	set := map[string]struct{}{}
	for _, c := range a {
		set[c.Path] = struct{}{}
	}
	var out []string
	for _, c := range b {
		if _, ok := set[c.Path]; ok {
			out = append(out, c.Path)
		}
	}
	sort.Strings(out)
	return out
}
