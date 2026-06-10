package cmd

import (
	"fmt"
	"sort"

	"github.com/abdul/bench/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage optional provider API keys",
	Long: `The supported agents (claude, codex, cursor-agent, opencode) each use their
own login — run ` + "`bench setup`" + ` to log in. This command only stores API keys
for any direct-API providers you wire up yourself.`,
}

var authSetCmd = &cobra.Command{
	Use:   "set <provider> <key>",
	Short: "Store an API key (e.g. bench auth set openai sk-...)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := config.LoadAuth()
		if err != nil {
			return err
		}
		a[args[0]] = args[1]
		if err := config.SaveAuth(a); err != nil {
			return err
		}
		fmt.Printf("stored key for %q\n", args[0])
		return nil
	},
}

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured providers (keys masked)",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := config.LoadAuth()
		if err != nil {
			return err
		}
		if len(a) == 0 {
			fmt.Println("No keys configured. claude/codex/cursor-agent/opencode each use their own login — run `bench setup`.")
			return nil
		}
		var keys []string
		for k := range a {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := a[k]
			mask := "****"
			if len(v) > 4 {
				mask = "****" + v[len(v)-4:]
			}
			fmt.Printf("• %-12s %s\n", k, mask)
		}
		return nil
	},
}

func init() {
	authCmd.AddCommand(authSetCmd, authListCmd)
}
