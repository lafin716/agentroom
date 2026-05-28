package copier

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentroom/agentroom/pkg/ignore"
)

// Stats describes a copy run.
type Stats struct {
	Files int
	Dirs  int
	Bytes int64
}

// CopyTree copies regular files from src into dst, skipping anything matched by m.
// Empty directories on the source are skipped (dst directories are created on demand).
func CopyTree(src, dst string, m *ignore.Matcher) (Stats, error) {
	var st Stats
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return st, err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return st, err
	}
	if err := os.MkdirAll(dstAbs, 0o755); err != nil {
		return st, err
	}

	err = filepath.WalkDir(srcAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == srcAbs {
			return nil
		}
		rel, err := filepath.Rel(srcAbs, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			if m != nil && m.Match(relSlash, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if m != nil && m.Match(relSlash, false) {
			return nil
		}
		target := filepath.Join(dstAbs, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		n, err := copyFile(path, target)
		if err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		st.Files++
		st.Bytes += n
		return nil
	})
	return st, err
}

func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-cp-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	n, err := io.Copy(tmp, in)
	if err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return n, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return n, err
	}
	_ = os.Chmod(tmpName, info.Mode().Perm())
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return n, err
	}
	return n, nil
}
