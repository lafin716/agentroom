package cmd

import (
	"github.com/agentroom/agentroom/pkg/logx"
	"github.com/agentroom/agentroom/pkg/syncer"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

func newUndoCmd() *cobra.Command {
	var steps int
	c := &cobra.Command{
		Use:   "undo",
		Short: "Revert the most recent sync (LIFO)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUndo(steps)
		},
	}
	c.Flags().IntVarP(&steps, "steps", "n", 1, "number of sync entries to undo")
	return c
}

func runUndo(steps int) error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	if steps < 1 {
		steps = 1
	}
	for i := 0; i < steps; i++ {
		man, err := syncer.Undo(mainPath)
		if err != nil {
			return err
		}
		logx.Infof("undone: %s (%d changes)", man.ID, len(man.Changes))
	}
	return nil
}
