package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is reported by `ar info` (overridable via -ldflags at build time).
var Version = "0.1.0"

var jsonOutput bool

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
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "emit machine-readable JSON instead of human output")
	rootCmd.AddCommand(
		newInitCmd(),
		newCopyCmd(),
		newStatusCmd(),
		newSyncCmd(),
		newUndoCmd(),
		newHistoryCmd(),
		newInfoCmd(),
		newResetCmd(),
		newMaskCmd(),
	)
}

// jsonEnabled reports whether the --json flag is set.
func jsonEnabled() bool { return jsonOutput }

// outputJSON writes v as pretty JSON to stdout.
func outputJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}
