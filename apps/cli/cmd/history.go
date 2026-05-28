package cmd

import (
	"fmt"

	"github.com/agentroom/agentroom/pkg/syncer"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

func newHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "List sync history (newest last)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory()
		},
	}
}

func runHistory() error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	h, err := syncer.LoadHistory(mainPath)
	if err != nil {
		return err
	}
	if len(h.Entries) == 0 {
		fmt.Println("(no syncs yet)")
		return nil
	}
	for i, e := range h.Entries {
		fmt.Printf("%2d  %s  %s  %s\n",
			i+1,
			e.ID,
			e.AppliedAt.Format("2006-01-02 15:04:05"),
			e.Summary,
		)
	}
	return nil
}
