package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/agentroom/agentroom/pkg/ignore"
	"github.com/agentroom/agentroom/pkg/mask"
)

// Entry is a single file's recorded state.
type Entry struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Mode   string `json:"mode"`
}

// Index maps repo-relative paths (slash-separated) to entries.
type Index struct {
	ComputedAt time.Time        `json:"computed_at"`
	Files      map[string]Entry `json:"files"`
}

// New builds an empty index.
func New() *Index {
	return &Index{ComputedAt: time.Now().UTC(), Files: map[string]Entry{}}
}

// Build walks root, hashes every non-ignored regular file, and returns the index.
func Build(root string, m *ignore.Matcher) (*Index, error) {
	return BuildWithMasks(root, m, nil)
}

// BuildWithMasks is like Build but, for any file with masked paths configured
// in masks, hashes the mask-applied content instead of the raw bytes. This
// normalization means workspace, main, and baseline all hash the same masked
// content — diffs ignore mask-only changes and only flag real edits.
func BuildWithMasks(root string, m *ignore.Matcher, masks *mask.Config) (*Index, error) {
	idx := New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if m != nil && m.Match(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if m != nil && m.Match(rel, false) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var sum string
		if paths := masks.PathsFor(rel); len(paths) > 0 {
			sum, err = hashMaskedFile(path, rel, paths)
		} else {
			sum, err = hashFile(path)
		}
		if err != nil {
			return fmt.Errorf("hash %s: %w", rel, err)
		}
		idx.Files[rel] = Entry{
			SHA256: sum,
			Size:   info.Size(),
			Mode:   fmt.Sprintf("%#o", info.Mode().Perm()),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

func hashMaskedFile(path, rel string, paths []string) (string, error) {
	kind := mask.ExtKind(rel)
	if kind == "" {
		// Mask config references a non-yaml/json file; fall back to raw hash.
		return hashFile(path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	masked, err := mask.Apply(raw, kind, paths)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(masked)
	return hex.EncodeToString(h[:]), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Save writes the index as pretty JSON to path.
func (i *Index) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads an index JSON file from path.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	if idx.Files == nil {
		idx.Files = map[string]Entry{}
	}
	return &idx, nil
}

// SortedPaths returns the file paths in stable order.
func (i *Index) SortedPaths() []string {
	out := make([]string, 0, len(i.Files))
	for p := range i.Files {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
