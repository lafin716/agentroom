package cmd

import (
	"fmt"

	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show workspace metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo()
		},
	}
}

func runInfo() error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	cfg, err := workspace.LoadConfig(mainPath)
	if err != nil {
		return err
	}
	fmt.Printf("project:   %s\n", cfg.ProjectName)
	fmt.Printf("version:   %d\n", cfg.Version)
	fmt.Printf("main:      %s\n", cfg.MainPath)
	if cfg.WorkspacePath == "" {
		fmt.Println("workspace: (none — run `ar copy`)")
	} else {
		fmt.Printf("workspace: %s\n", cfg.WorkspacePath)
	}
	fmt.Printf("created:   %s\n", cfg.CreatedAt.Format("2006-01-02 15:04:05 MST"))
	if !cfg.LastSyncAt.IsZero() {
		fmt.Printf("last sync: %s\n", cfg.LastSyncAt.Format("2006-01-02 15:04:05 MST"))
	}
	return nil
}
