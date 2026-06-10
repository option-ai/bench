package cmd

import (
	"fmt"

	"github.com/abdul/bench/internal/config"
	"github.com/abdul/bench/internal/skill"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the /add-to-bench skill globally and set up config dirs",
	Long:  "Non-interactive setup. For a guided walkthrough (agent logins + judge), use `bench setup`.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.Load(); err != nil { // ensures dirs + writes default config.json
			return err
		}
		dest, err := skill.Install()
		if err != nil {
			return err
		}
		fmt.Printf("✓ installed skill   %s\n", dest)
		fmt.Printf("✓ config ready      %s\n", config.Dir())
		fmt.Println("\nNext: `bench setup` to check agent logins and pick a judge, or /add-to-bench inside Claude Code.")
		return nil
	},
}
