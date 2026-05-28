package syncer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentroom/agentroom/pkg/differ"
	"github.com/agentroom/agentroom/pkg/workspace"
)

// HistoryEntry is the LIFO record per sync.
type HistoryEntry struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	AppliedAt time.Time `json:"applied_at"`
}

// HistoryIndex is the stack file: index.json under history/.
type HistoryIndex struct {
	Entries []HistoryEntry `json:"entries"`
}

// ChangeRecord is one file change captured in a manifest.
type ChangeRecord struct {
	Path      string     `json:"path"`
	Op        differ.Op  `json:"op"`
	HadBackup bool       `json:"had_backup"`
}

// Manifest is the per-entry detail file written into history/<id>/manifest.json.
type Manifest struct {
	ID             string         `json:"id"`
	AppliedAt      time.Time      `json:"applied_at"`
	Changes        []ChangeRecord `json:"changes"`
	BaselineBefore string         `json:"baseline_before"`
}

func LoadHistory(mainPath string) (*HistoryIndex, error) {
	path := workspace.HistoryIndexPath(mainPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &HistoryIndex{}, nil
		}
		return nil, err
	}
	var h HistoryIndex
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parse history index: %w", err)
	}
	return &h, nil
}

func SaveHistory(mainPath string, h *HistoryIndex) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return workspace.AtomicWrite(workspace.HistoryIndexPath(mainPath), data, 0o644)
}

func EntryDir(mainPath, id string) string {
	return filepath.Join(workspace.HistoryPath(mainPath), id)
}

func WriteManifest(mainPath string, m *Manifest) error {
	dir := EntryDir(mainPath, m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return workspace.AtomicWrite(filepath.Join(dir, "manifest.json"), data, 0o644)
}

func ReadManifest(mainPath, id string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(EntryDir(mainPath, id), "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// NewEntryID generates a sortable, unique id.
func NewEntryID(seq int) string {
	return fmt.Sprintf("%04d-%s", seq, time.Now().UTC().Format("20060102T150405Z"))
}
