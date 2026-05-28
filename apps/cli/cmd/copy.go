package cmd

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentroom/agentroom/pkg/copier"
	"github.com/agentroom/agentroom/pkg/ignore"
	"github.com/agentroom/agentroom/pkg/index"
	"github.com/agentroom/agentroom/pkg/logx"
	"github.com/agentroom/agentroom/pkg/mask"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

type copyResult struct {
	WorkspacePath string `json:"workspace_path"`
	MainPath      string `json:"main_path"`
	FilesCopied   int    `json:"files_copied"`
	BytesCopied   int64  `json:"bytes_copied"`
	BaselinePath  string `json:"baseline_path"`
	FilesIndexed  int    `json:"files_indexed"`
}

func newCopyCmd() *cobra.Command {
	var dest string
	var force bool
	c := &cobra.Command{
		Use:   "copy",
		Short: "Create the sanitized workspace copy of this project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(dest, force)
		},
	}
	c.Flags().StringVar(&dest, "dest", "", "destination path (default: ~/.agentroom/workspaces/<name>-<hash>/)")
	c.Flags().BoolVar(&force, "force", false, "overwrite existing destination")
	return c
}

func runCopy(dest string, force bool) error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	cfg, err := workspace.LoadConfig(mainPath)
	if err != nil {
		return err
	}

	if dest == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		h := sha1.Sum([]byte(mainPath))
		dest = filepath.Join(home, ".agentroom", "workspaces",
			fmt.Sprintf("%s-%s", filepath.Base(mainPath), hex.EncodeToString(h[:4])))
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	if info, err := os.Stat(destAbs); err == nil {
		if !force {
			return fmt.Errorf("destination already exists: %s (use --force to overwrite)", destAbs)
		}
		if !info.IsDir() {
			return fmt.Errorf("destination exists and is not a directory: %s", destAbs)
		}
		if err := os.RemoveAll(destAbs); err != nil {
			return err
		}
	}

	matcher, err := ignore.LoadFromFile(workspace.IgnorePath(mainPath))
	if err != nil {
		return err
	}

	if !jsonEnabled() {
		logx.Infof("copying %s -> %s", mainPath, destAbs)
	}
	st, err := copier.CopyTree(mainPath, destAbs, matcher)
	if err != nil {
		return err
	}

	masksCfg, err := mask.Load(mainPath)
	if err != nil {
		return err
	}
	masked, err := applyMasksToWorkspace(destAbs, masksCfg)
	if err != nil {
		return fmt.Errorf("apply masks: %w", err)
	}
	if !jsonEnabled() {
		logx.Infof("copied %d files (%d bytes)", st.Files, st.Bytes)
		if masked > 0 {
			logx.Infof("masked %d file(s) in workspace", masked)
		}
		logx.Infof("building baseline...")
	}

	idx, err := index.BuildWithMasks(mainPath, matcher, masksCfg)
	if err != nil {
		return err
	}
	if err := idx.Save(workspace.BaselinePath(mainPath)); err != nil {
		return err
	}
	if !jsonEnabled() {
		logx.Infof("baseline: %d files indexed", len(idx.Files))
	}

	marker := workspace.WorkspaceMarkerFile{MainPath: mainPath, CreatedAt: time.Now().UTC()}
	data, _ := json.MarshalIndent(&marker, "", "  ")
	if err := os.WriteFile(workspace.MarkerPath(destAbs), data, 0o644); err != nil {
		return err
	}

	cfg.WorkspacePath = destAbs
	if err := workspace.SaveConfig(mainPath, cfg); err != nil {
		return err
	}

	if jsonEnabled() {
		return outputJSON(copyResult{
			WorkspacePath: destAbs,
			MainPath:      mainPath,
			FilesCopied:   st.Files,
			BytesCopied:   st.Bytes,
			BaselinePath:  workspace.BaselinePath(mainPath),
			FilesIndexed:  len(idx.Files),
		})
	}
	logx.Infof("workspace ready: %s", destAbs)
	return nil
}

// applyMasksToWorkspace rewrites each configured mask file inside the
// workspace, replacing masked paths with the placeholder. Returns the number
// of files modified.
func applyMasksToWorkspace(workspacePath string, cfg *mask.Config) (int, error) {
	if cfg == nil || len(cfg.Masks) == 0 {
		return 0, nil
	}
	count := 0
	for _, m := range cfg.Masks {
		kind := mask.ExtKind(m.File)
		if kind == "" {
			continue
		}
		wsFile := filepath.Join(workspacePath, filepath.FromSlash(m.File))
		raw, err := os.ReadFile(wsFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return count, err
		}
		out, err := mask.Apply(raw, kind, m.Paths)
		if err != nil {
			return count, fmt.Errorf("%s: %w", m.File, err)
		}
		info, _ := os.Stat(wsFile)
		mode := os.FileMode(0o644)
		if info != nil {
			mode = info.Mode().Perm()
		}
		if err := workspace.AtomicWrite(wsFile, out, mode); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
