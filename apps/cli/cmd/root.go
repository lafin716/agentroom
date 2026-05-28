package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ar",
	Short: "agentroom — safely use coding agents on internal projects",
	Long: `agentroom (ar) creates a sanitized copy of your project where coding
agents can work without exposure to .env / API keys, then syncs changes
back to the main project with conflict detection and undo support.`,
	SilenceUsage: true,
}

// Execute runs the root command and returns any error.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(
		newInitCmd(),
		newCopyCmd(),
		newStatusCmd(),
		newSyncCmd(),
		newUndoCmd(),
		newHistoryCmd(),
		newInfoCmd(),
	)
}
