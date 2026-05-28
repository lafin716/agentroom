package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	AgentDir         = ".agent"
	ConfigFile       = "config.json"
	BaselineFile     = "baseline.json"
	HistoryDir       = "history"
	HistoryIndexFile = "index.json"
	IgnoreFile       = ".agentignore"
	MasksFile        = "masks.json"
	WorkspaceMarker  = ".agent-workspace.json"
	ConfigVersion    = 1
)

type Config struct {
	Version       int       `json:"version"`
	ProjectName   string    `json:"project_name"`
	MainPath      string    `json:"main_path"`
	WorkspacePath string    `json:"workspace_path,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	LastSyncAt    time.Time `json:"last_sync_at,omitempty"`
}

type WorkspaceMarkerFile struct {
	MainPath  string    `json:"main_path"`
	CreatedAt time.Time `json:"created_at"`
}

func AgentPath(mainPath string) string {
	return filepath.Join(mainPath, AgentDir)
}

func ConfigPath(mainPath string) string {
	return filepath.Join(AgentPath(mainPath), ConfigFile)
}

func BaselinePath(mainPath string) string {
	return filepath.Join(AgentPath(mainPath), BaselineFile)
}

func HistoryPath(mainPath string) string {
	return filepath.Join(AgentPath(mainPath), HistoryDir)
}

func HistoryIndexPath(mainPath string) string {
	return filepath.Join(HistoryPath(mainPath), HistoryIndexFile)
}

func IgnorePath(mainPath string) string {
	return filepath.Join(mainPath, IgnoreFile)
}

func MasksPath(mainPath string) string {
	return filepath.Join(AgentPath(mainPath), MasksFile)
}

func MarkerPath(workspacePath string) string {
	return filepath.Join(workspacePath, WorkspaceMarker)
}

func LoadConfig(mainPath string) (*Config, error) {
	data, err := os.ReadFile(ConfigPath(mainPath))
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func SaveConfig(mainPath string, cfg *Config) error {
	if err := os.MkdirAll(AgentPath(mainPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(ConfigPath(mainPath), data, 0o644)
}

func IsInitialized(mainPath string) bool {
	_, err := os.Stat(ConfigPath(mainPath))
	return err == nil
}

// FindMainRoot walks up from start to find a directory containing .agent/config.json.
func FindMainRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	cur := abs
	for {
		if _, err := os.Stat(filepath.Join(cur, AgentDir, ConfigFile)); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", errors.New("agentroom not initialized (no .agent/config.json found in any parent)")
		}
		cur = parent
	}
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		// non-fatal on Windows
		_ = err
	}
	return os.Rename(tmpName, path)
}

// AtomicWrite is the exported helper for other packages.
func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	return atomicWrite(path, data, mode)
}
