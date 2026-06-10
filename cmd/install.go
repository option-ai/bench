package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abdul/bench/internal/config"
	"github.com/abdul/bench/internal/skill"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the /add-to-bench skill globally and set up config dirs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.EnsureDirs(); err != nil {
			return err
		}
		if _, err := config.Load(); err != nil { // writes default config.json
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir := filepath.Join(home, ".claude", "skills", skill.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		dest := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(dest, []byte(skill.Markdown), 0o644); err != nil {
			return err
		}
		fmt.Printf("✓ installed skill   %s\n", dest)
		fmt.Printf("✓ config ready      %s\n", config.Dir())
		fmt.Println("\nNext: capture an eval with /add-to-bench inside Claude Code, then `bench run`.")
		return nil
	},
}
