package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/agentroom/agentroom/pkg/logx"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

type resetResult struct {
	WorkspaceDeleted bool   `json:"workspace_deleted"`
	WorkspacePath    string `json:"workspace_path"`
	BaselineDeleted  bool   `json:"baseline_deleted"`
	HistoryDeleted   bool   `json:"history_deleted"`
}

func newResetCmd() *cobra.Command {
	var yes bool
	var withHistory bool
	c := &cobra.Command{
		Use:   "reset",
		Short: "Delete the workspace copy and clear baseline (optionally history)",
		Long: `Removes the workspace copy at config.workspace_path, deletes .agent/baseline.json,
and clears workspace_path in config so the next 'ar copy' starts fresh.
With --with-history, also deletes .agent/history/ (irreversible: past syncs can no longer be undone).
The main project tree is never touched.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReset(yes, withHistory)
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	c.Flags().BoolVar(&withHistory, "with-history", false, "also delete .agent/history/")
	return c
}

func runReset(yes, withHistory bool) error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	cfg, err := workspace.LoadConfig(mainPath)
	if err != nil {
		return err
	}

	wsPath := cfg.WorkspacePath
	if !yes && !jsonEnabled() {
		fmt.Printf("This will delete:\n")
		if wsPath != "" {
			fmt.Printf("  - workspace: %s\n", wsPath)
		}
		fmt.Printf("  - %s\n", workspace.BaselinePath(mainPath))
		if withHistory {
			fmt.Printf("  - %s  (sync history; undo will no longer work)\n", workspace.HistoryPath(mainPath))
		}
		fmt.Print("Continue? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(line), "y") {
			return fmt.Errorf("aborted")
		}
	}

	res := resetResult{WorkspacePath: wsPath}

	if wsPath != "" {
		if _, err := os.Stat(wsPath); err == nil {
			if err := os.RemoveAll(wsPath); err != nil {
				return fmt.Errorf("delete workspace: %w", err)
			}
			res.WorkspaceDeleted = true
		}
	}

	baseline := workspace.BaselinePath(mainPath)
	if _, err := os.Stat(baseline); err == nil {
		if err := os.Remove(baseline); err != nil {
			return fmt.Errorf("delete baseline: %w", err)
		}
		res.BaselineDeleted = true
	}

	if withHistory {
		hist := workspace.HistoryPath(mainPath)
		if _, err := os.Stat(hist); err == nil {
			if err := os.RemoveAll(hist); err != nil {
				return fmt.Errorf("delete history: %w", err)
			}
			res.HistoryDeleted = true
		}
		cfg.LastSyncAt = time.Time{}
	}

	cfg.WorkspacePath = ""
	if err := workspace.SaveConfig(mainPath, cfg); err != nil {
		return err
	}

	if jsonEnabled() {
		return outputJSON(res)
	}
	if res.WorkspaceDeleted {
		logx.Infof("workspace deleted: %s", wsPath)
	}
	if res.BaselineDeleted {
		logx.Infof("baseline deleted")
	}
	if res.HistoryDeleted {
		logx.Infof("history deleted")
	}
	logx.Infof("reset complete")
	return nil
}
