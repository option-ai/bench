// Package cmd wires the bench CLI: run, list, auth, install.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "bench",
	Short: "A personal benchmark for coding agents, seeded from your real sessions",
	Long: `bench replays the prompts from your real coding sessions against multiple
agents on the exact repo state you captured, then a blind judge scores each
diff into a single composite number so you can compare models head-to-head.

Capture evals with the /add-to-bench skill inside Claude Code, then run them here.`,
}

// Execute runs the root command.
func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	root.AddCommand(setupCmd, listCmd, runCmd, authCmd, installCmd)
}
