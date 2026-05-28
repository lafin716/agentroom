package cmd

import (
	"fmt"
	"time"

	"github.com/agentroom/agentroom/pkg/syncer"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

type historyEntryJSON struct {
	Seq       int       `json:"seq"`
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	AppliedAt time.Time `json:"applied_at"`
}

type historyResult struct {
	Entries []historyEntryJSON `json:"entries"`
}

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

	if jsonEnabled() {
		res := historyResult{Entries: make([]historyEntryJSON, 0, len(h.Entries))}
		for i, e := range h.Entries {
			res.Entries = append(res.Entries, historyEntryJSON{
				Seq:       i + 1,
				ID:        e.ID,
				Summary:   e.Summary,
				AppliedAt: e.AppliedAt,
			})
		}
		return outputJSON(res)
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
