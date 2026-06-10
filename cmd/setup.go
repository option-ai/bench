package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/abdul/bench/internal/adapter"
	"github.com/abdul/bench/internal/config"
	"github.com/abdul/bench/internal/skill"
	"github.com/abdul/bench/internal/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	sTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	sSect  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75")).MarginTop(1)
	sOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("84"))
	sWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	sDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Guided setup: install the skill, check agent logins, pick your judge",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(sTitle.Render("bench setup"))

		// 1. config + skill
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		dest, err := skill.Install()
		if err != nil {
			return err
		}
		fmt.Println(sSect.Render("Workspace"))
		fmt.Printf("  %s config   %s\n", sOK.Render("✓"), config.Dir())
		fmt.Printf("  %s skill    %s\n", sOK.Render("✓"), dest)

		// 2. agents + login (each agent uses its own login, not an API key)
		fmt.Println(sSect.Render("Coding agents"))
		fmt.Println(sDim.Render("  Each agent authenticates with its own login — bench stores no API keys."))
		in := bufio.NewReader(os.Stdin)
		anyAvailable := false
		for _, a := range adapter.All() {
			if !a.Available() {
				fmt.Printf("  %s %-13s not found on PATH\n", sWarn.Render("•"), a.ID())
				continue
			}
			anyAvailable = true
			ai := a.Auth()
			fmt.Printf("  %s %-13s installed — %s\n", sOK.Render("✓"), a.ID(), sDim.Render(ai.Note))
			if ai.LoginCmd != "" && confirm(in, fmt.Sprintf("      log in now with `%s`?", ai.LoginCmd)) {
				if err := runLogin(ai.LoginCmd); err != nil {
					fmt.Printf("      %s login exited: %v\n", sWarn.Render("!"), err)
				}
			}
		}
		if !anyAvailable {
			fmt.Println(sWarn.Render("\n  No agents found. Install at least one of: claude, codex, cursor-agent, opencode."))
			return nil
		}

		// 3. default judge
		fmt.Println(sSect.Render("Judge"))
		models := adapter.AvailableModelsWith(cfg.Models)
		items := make([]tui.Item, len(models))
		for i, m := range models {
			items[i] = tui.Item{Label: m.Ref(), Desc: m.Agent}
		}
		idx, err := tui.PickOne("Choose your default judge (blind grader — sees only the diff + rubric)", items)
		if err != nil {
			fmt.Printf("  %s kept existing default judge: %s\n", sDim.Render("·"), cfg.DefaultJudge)
		} else {
			cfg.DefaultJudge = models[idx].Ref()
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("  %s default judge: %s\n", sOK.Render("✓"), cfg.DefaultJudge)
		}

		// 4. summary
		fmt.Println(sSect.Render("Ready"))
		fmt.Println("  • Capture evals with /add-to-bench inside Claude Code (new session to load the skill)")
		fmt.Println("  • Run them with `bench run`")
		fmt.Println("  • Tune scoring in " + config.Dir() + "/config.json")
		return nil
	},
}

// confirm reads a y/N answer from stdin (default no).
func confirm(in *bufio.Reader, prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

// runLogin execs a login command inheriting the terminal, so the agent's own
// interactive login flow runs inline.
func runLogin(cmdline string) error {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return nil
	}
	c := exec.Command(parts[0], parts[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}
