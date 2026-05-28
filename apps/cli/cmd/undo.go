package cmd

import (
	"github.com/agentroom/agentroom/pkg/logx"
	"github.com/agentroom/agentroom/pkg/syncer"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

type undoneEntry struct {
	ID           string `json:"id"`
	ChangesCount int    `json:"changes_count"`
}

type undoResult struct {
	Undone    []undoneEntry `json:"undone"`
	Remaining int           `json:"remaining"`
}

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
	res := undoResult{Undone: []undoneEntry{}}
	for i := 0; i < steps; i++ {
		man, err := syncer.Undo(mainPath)
		if err != nil {
			return err
		}
		res.Undone = append(res.Undone, undoneEntry{ID: man.ID, ChangesCount: len(man.Changes)})
		if !jsonEnabled() {
			logx.Infof("undone: %s (%d changes)", man.ID, len(man.Changes))
		}
	}
	if jsonEnabled() {
		h, err := syncer.LoadHistory(mainPath)
		if err != nil {
			return err
		}
		res.Remaining = len(h.Entries)
		return outputJSON(res)
	}
	return nil
}
