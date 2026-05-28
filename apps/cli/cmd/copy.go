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
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

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

	logx.Infof("copying %s -> %s", mainPath, destAbs)
	st, err := copier.CopyTree(mainPath, destAbs, matcher)
	if err != nil {
		return err
	}
	logx.Infof("copied %d files (%d bytes)", st.Files, st.Bytes)

	// Build baseline from the main project (post-ignore) so future diffs are consistent.
	logx.Infof("building baseline...")
	idx, err := index.Build(mainPath, matcher)
	if err != nil {
		return err
	}
	if err := idx.Save(workspace.BaselinePath(mainPath)); err != nil {
		return err
	}
	logx.Infof("baseline: %d files indexed", len(idx.Files))

	// Workspace marker (lets us back-reference the main project later).
	marker := workspace.WorkspaceMarkerFile{MainPath: mainPath, CreatedAt: time.Now().UTC()}
	data, _ := json.MarshalIndent(&marker, "", "  ")
	if err := os.WriteFile(workspace.MarkerPath(destAbs), data, 0o644); err != nil {
		return err
	}

	cfg.WorkspacePath = destAbs
	if err := workspace.SaveConfig(mainPath, cfg); err != nil {
		return err
	}
	logx.Infof("workspace ready: %s", destAbs)
	return nil
}
