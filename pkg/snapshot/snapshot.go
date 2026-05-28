package snapshot

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Snapshot represents a per-sync backup directory: <root>/files/<orig path>.
type Snapshot struct {
	Root string
}

// New creates the snapshot root directory.
func New(root string) (*Snapshot, error) {
	if err := os.MkdirAll(filepath.Join(root, "files"), 0o755); err != nil {
		return nil, err
	}
	return &Snapshot{Root: root}, nil
}

func (s *Snapshot) FilesDir() string { return filepath.Join(s.Root, "files") }

// Backup copies the file at absSource into the snapshot, preserving relPath layout.
// Returns true if a backup was created, false if the source doesn't exist.
func (s *Snapshot) Backup(absSource, relPath string) (bool, error) {
	info, err := os.Stat(absSource)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	dst := filepath.Join(s.FilesDir(), filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}
	in, err := os.Open(absSource)
	if err != nil {
		return false, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return false, err
	}
	if err := out.Close(); err != nil {
		return false, err
	}
	_ = os.Chmod(dst, info.Mode().Perm())
	return true, nil
}

// Restore copies the backed up file back to absTarget. Returns os.ErrNotExist if no backup.
func (s *Snapshot) Restore(relPath, absTarget string) error {
	src := filepath.Join(s.FilesDir(), filepath.FromSlash(relPath))
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(absTarget), ".tmp-rs-*")
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
	if err := os.Rename(tmpName, absTarget); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("restore %s: %w", relPath, err)
	}
	return nil
}
