package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/agentroom/agentroom/pkg/logx"
	"github.com/agentroom/agentroom/pkg/syncer"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var yes, force, withDelete bool
	c := &cobra.Command{
		Use:   "sync",
		Short: "Apply workspace changes back to the main project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(yes, force, withDelete)
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	c.Flags().BoolVar(&force, "force", false, "apply even if main drifted (overwrites)")
	c.Flags().BoolVar(&withDelete, "with-delete", true, "propagate file deletions to main")
	return c
}

func runSync(yes, force, withDelete bool) error {
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
	if !plan.HasWork() {
		fmt.Println("nothing to sync.")
		return nil
	}

	printChanges("workspace -> main", plan.WorkspaceChanges)
	if len(plan.Conflicts) > 0 {
		fmt.Println()
		fmt.Printf("conflicts (changed in both): %d\n", len(plan.Conflicts))
		for _, p := range plan.Conflicts {
			fmt.Printf("  ! %s\n", p)
		}
		if !force {
			return fmt.Errorf("sync aborted; resolve conflicts or use --force")
		}
		fmt.Println("--force given; conflicts will be overwritten.")
	}

	if !yes {
		fmt.Print("\nApply these changes? [y/N]: ")
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			fmt.Println("aborted.")
			return nil
		}
	}

	man, err := syncer.Apply(syncer.ApplyOptions{
		MainPath:      mainPath,
		WorkspacePath: cfg.WorkspacePath,
		Plan:          plan,
		WithDelete:    withDelete,
		Force:         force,
	})
	if err != nil {
		return err
	}
	logx.Infof("sync applied: %s (%d changes)", man.ID, len(man.Changes))
	return nil
}
