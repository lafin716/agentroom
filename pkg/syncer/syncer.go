package syncer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/agentroom/agentroom/pkg/differ"
	"github.com/agentroom/agentroom/pkg/ignore"
	"github.com/agentroom/agentroom/pkg/index"
	"github.com/agentroom/agentroom/pkg/snapshot"
	"github.com/agentroom/agentroom/pkg/workspace"
)

// Plan is the result of analyzing a potential sync.
type Plan struct {
	WorkspaceChanges []differ.Change // copy vs baseline (what the user wants to apply)
	MainChanges      []differ.Change // main vs baseline (used to detect conflicts)
	Conflicts        []string        // intersection: changed in both
}

// HasWork returns true if there is anything to sync.
func (p *Plan) HasWork() bool { return len(p.WorkspaceChanges) > 0 }

// AnalyzeOptions controls Analyze().
type AnalyzeOptions struct {
	MainPath      string
	WorkspacePath string
}

// Analyze computes a sync plan: compares workspace and main against the stored baseline.
func Analyze(opt AnalyzeOptions) (*Plan, error) {
	baseline, err := index.Load(workspace.BaselinePath(opt.MainPath))
	if err != nil {
		return nil, fmt.Errorf("load baseline: %w", err)
	}
	matcher, err := ignore.LoadFromFile(workspace.IgnorePath(opt.MainPath))
	if err != nil {
		return nil, fmt.Errorf("load .agentignore: %w", err)
	}
	wsIdx, err := index.Build(opt.WorkspacePath, matcher)
	if err != nil {
		return nil, fmt.Errorf("scan workspace: %w", err)
	}
	mainIdx, err := index.Build(opt.MainPath, matcher)
	if err != nil {
		return nil, fmt.Errorf("scan main: %w", err)
	}
	wsChanges := differ.Diff(baseline, wsIdx)
	mainChanges := differ.Diff(baseline, mainIdx)
	conflicts := differ.Conflicts(wsChanges, mainChanges)
	return &Plan{
		WorkspaceChanges: wsChanges,
		MainChanges:      mainChanges,
		Conflicts:        conflicts,
	}, nil
}

// ApplyOptions controls Apply().
type ApplyOptions struct {
	MainPath      string
	WorkspacePath string
	Plan          *Plan
	WithDelete    bool
	Force         bool // if true, ignore conflicts
}

// Apply executes the plan: backup → write changes → update baseline → record history.
func Apply(opt ApplyOptions) (*Manifest, error) {
	if opt.Plan == nil {
		return nil, errors.New("nil plan")
	}
	if len(opt.Plan.Conflicts) > 0 && !opt.Force {
		return nil, fmt.Errorf("aborting: %d conflict(s) detected", len(opt.Plan.Conflicts))
	}

	hist, err := LoadHistory(opt.MainPath)
	if err != nil {
		return nil, err
	}
	seq := len(hist.Entries) + 1
	id := NewEntryID(seq)
	entryDir := EntryDir(opt.MainPath, id)
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		return nil, err
	}
	snap, err := snapshot.New(entryDir)
	if err != nil {
		return nil, err
	}

	// Save baseline-before for undo.
	baselineSrc := workspace.BaselinePath(opt.MainPath)
	baselineBackupName := "baseline-before.json"
	if err := copyFile(baselineSrc, filepath.Join(entryDir, baselineBackupName)); err != nil {
		return nil, fmt.Errorf("backup baseline: %w", err)
	}

	manifest := &Manifest{
		ID:             id,
		AppliedAt:      time.Now().UTC(),
		BaselineBefore: baselineBackupName,
	}

	// Step 1: backup every file the change touches on the main side.
	for _, c := range opt.Plan.WorkspaceChanges {
		if c.Op == differ.OpDeleted && !opt.WithDelete {
			continue
		}
		mainFile := filepath.Join(opt.MainPath, filepath.FromSlash(c.Path))
		had, err := snap.Backup(mainFile, c.Path)
		if err != nil {
			return nil, fmt.Errorf("backup %s: %w", c.Path, err)
		}
		manifest.Changes = append(manifest.Changes, ChangeRecord{
			Path:      c.Path,
			Op:        c.Op,
			HadBackup: had,
		})
	}

	// Step 2: apply changes from workspace -> main.
	for _, rec := range manifest.Changes {
		mainFile := filepath.Join(opt.MainPath, filepath.FromSlash(rec.Path))
		wsFile := filepath.Join(opt.WorkspacePath, filepath.FromSlash(rec.Path))
		switch rec.Op {
		case differ.OpAdded, differ.OpModified:
			if err := os.MkdirAll(filepath.Dir(mainFile), 0o755); err != nil {
				return nil, err
			}
			if err := copyFile(wsFile, mainFile); err != nil {
				return nil, fmt.Errorf("write %s: %w", rec.Path, err)
			}
		case differ.OpDeleted:
			if err := os.Remove(mainFile); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("delete %s: %w", rec.Path, err)
			}
		}
	}

	// Step 3: rebuild baseline from current main state.
	matcher, err := ignore.LoadFromFile(workspace.IgnorePath(opt.MainPath))
	if err != nil {
		return nil, err
	}
	newBaseline, err := index.Build(opt.MainPath, matcher)
	if err != nil {
		return nil, err
	}
	if err := newBaseline.Save(workspace.BaselinePath(opt.MainPath)); err != nil {
		return nil, err
	}

	// Step 4: write manifest + history index.
	if err := WriteManifest(opt.MainPath, manifest); err != nil {
		return nil, err
	}
	hist.Entries = append(hist.Entries, HistoryEntry{
		ID:        id,
		Summary:   fmt.Sprintf("%d files changed", len(manifest.Changes)),
		AppliedAt: manifest.AppliedAt,
	})
	if err := SaveHistory(opt.MainPath, hist); err != nil {
		return nil, err
	}

	// Step 5: update config last_sync_at.
	cfg, err := workspace.LoadConfig(opt.MainPath)
	if err == nil {
		cfg.LastSyncAt = manifest.AppliedAt
		_ = workspace.SaveConfig(opt.MainPath, cfg)
	}

	return manifest, nil
}

// Undo reverses the most recent history entry on the stack.
func Undo(mainPath string) (*Manifest, error) {
	hist, err := LoadHistory(mainPath)
	if err != nil {
		return nil, err
	}
	if len(hist.Entries) == 0 {
		return nil, errors.New("no sync history to undo")
	}
	top := hist.Entries[len(hist.Entries)-1]
	man, err := ReadManifest(mainPath, top.ID)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	snap := &snapshot.Snapshot{Root: EntryDir(mainPath, top.ID)}

	// Reverse each change.
	for _, rec := range man.Changes {
		mainFile := filepath.Join(mainPath, filepath.FromSlash(rec.Path))
		switch rec.Op {
		case differ.OpAdded:
			// Added by sync → remove from main.
			if err := os.Remove(mainFile); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("undo add %s: %w", rec.Path, err)
			}
		case differ.OpModified, differ.OpDeleted:
			if !rec.HadBackup {
				continue
			}
			if err := snap.Restore(rec.Path, mainFile); err != nil {
				return nil, fmt.Errorf("undo %s %s: %w", rec.Op, rec.Path, err)
			}
		}
	}

	// Restore baseline.
	src := filepath.Join(EntryDir(mainPath, top.ID), man.BaselineBefore)
	if err := copyFile(src, workspace.BaselinePath(mainPath)); err != nil {
		return nil, fmt.Errorf("restore baseline: %w", err)
	}

	// Pop history.
	hist.Entries = hist.Entries[:len(hist.Entries)-1]
	if err := SaveHistory(mainPath, hist); err != nil {
		return nil, err
	}

	// Remove the entry's directory (history of this entry is no longer needed).
	_ = os.RemoveAll(EntryDir(mainPath, top.ID))

	return man, nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-sy-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	_ = os.Chmod(tmpName, info.Mode().Perm())
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
