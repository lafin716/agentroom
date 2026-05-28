package cmd

import (
	"fmt"
	"time"

	"github.com/agentroom/agentroom/pkg/differ"
	"github.com/agentroom/agentroom/pkg/syncer"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

type changeJSON struct {
	Op   string `json:"op"`
	Path string `json:"path"`
}

type statusResult struct {
	Main             string       `json:"main"`
	Workspace        string       `json:"workspace"`
	LastSync         *time.Time   `json:"last_sync,omitempty"`
	WorkspaceChanges []changeJSON `json:"workspace_changes"`
	MainChanges      []changeJSON `json:"main_changes"`
	Conflicts        []string     `json:"conflicts"`
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show pending changes in the workspace and on main",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}
}

func runStatus() error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	cfg, err := workspace.LoadConfig(mainPath)
	if err != nil {
		return err
	}
	if cfg.WorkspacePath == "" {
		return fmt.Errorf("no workspace yet; run `ar copy` first")
	}

	plan, err := syncer.Analyze(syncer.AnalyzeOptions{
		MainPath:      mainPath,
		WorkspacePath: cfg.WorkspacePath,
	})
	if err != nil {
		return err
	}

	if jsonEnabled() {
		res := statusResult{
			Main:             mainPath,
			Workspace:        cfg.WorkspacePath,
			WorkspaceChanges: toChangeJSON(plan.WorkspaceChanges),
			MainChanges:      toChangeJSON(plan.MainChanges),
			Conflicts:        plan.Conflicts,
		}
		if res.Conflicts == nil {
			res.Conflicts = []string{}
		}
		if !cfg.LastSyncAt.IsZero() {
			t := cfg.LastSyncAt
			res.LastSync = &t
		}
		return outputJSON(res)
	}

	fmt.Printf("main:      %s\n", mainPath)
	fmt.Printf("workspace: %s\n", cfg.WorkspacePath)
	if !cfg.LastSyncAt.IsZero() {
		fmt.Printf("last sync: %s\n", cfg.LastSyncAt.Format("2006-01-02 15:04:05 MST"))
	}
	fmt.Println()

	printChanges("workspace -> main (pending sync)", plan.WorkspaceChanges)
	fmt.Println()
	printChanges("main drift since last sync", plan.MainChanges)

	if len(plan.Conflicts) > 0 {
		fmt.Println()
		fmt.Printf("conflicts (changed in both): %d\n", len(plan.Conflicts))
		for _, p := range plan.Conflicts {
			fmt.Printf("  ! %s\n", p)
		}
	}
	return nil
}

func toChangeJSON(cs []differ.Change) []changeJSON {
	out := make([]changeJSON, 0, len(cs))
	for _, c := range cs {
		out = append(out, changeJSON{Op: string(c.Op), Path: c.Path})
	}
	return out
}

func printChanges(title string, cs []differ.Change) {
	fmt.Printf("%s:\n", title)
	if len(cs) == 0 {
		fmt.Println("  (no changes)")
		return
	}
	for _, c := range cs {
		var marker string
		switch c.Op {
		case differ.OpAdded:
			marker = "+"
		case differ.OpModified:
			marker = "~"
		case differ.OpDeleted:
			marker = "-"
		}
		fmt.Printf("  %s %s\n", marker, c.Path)
	}
}
