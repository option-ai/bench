package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/abdul/bench/internal/adapter"
	"github.com/abdul/bench/internal/config"
	"github.com/abdul/bench/internal/runner"
	"github.com/abdul/bench/internal/score"
	"github.com/abdul/bench/internal/snapshot"
	"github.com/abdul/bench/internal/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	flagTimeout   time.Duration
	flagJudgeTO   time.Duration
	flagJudgeRef  string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Select evals + models, run them, and score into a leaderboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		snaps, err := snapshot.LoadAll()
		if err != nil {
			return err
		}
		if len(snaps) == 0 {
			fmt.Println("No evals yet. Capture one with /add-to-bench inside Claude Code.")
			return nil
		}
		models := adapter.AvailableModels()
		if len(models) == 0 {
			fmt.Println("No coding agents found on PATH (claude, codex, cursor-agent, opencode).")
			return nil
		}

		// 1. pick evals
		evalItems := make([]tui.Item, len(snaps))
		for i, s := range snaps {
			evalItems[i] = tui.Item{Label: s.Title, Desc: fmt.Sprintf("%s@%.8s", s.Repo, s.Commit)}
		}
		ei, err := tui.PickMany("Select evals to run", evalItems)
		if err != nil {
			return err
		}
		selEvals := pick(snaps, ei)

		// 2. pick models
		modelItems := make([]tui.Item, len(models))
		for i, m := range models {
			modelItems[i] = tui.Item{Label: m.Ref(), Desc: m.Agent}
		}
		mi, err := tui.PickMany("Select models", modelItems)
		if err != nil {
			return err
		}
		selModels := pick(models, mi)

		// 3. pick judge (single, blind grader)
		judge := models[0]
		if flagJudgeRef != "" {
			j, err := adapter.ParseRef(flagJudgeRef)
			if err != nil {
				return err
			}
			judge = j
		} else {
			ji, err := tui.PickOne("Select the judge (blind grader — sees only the diff + rubric)", modelItems)
			if err != nil {
				return err
			}
			judge = models[ji]
		}

		// 4. run with live progress
		fmt.Printf("\nRunning %d eval(s) × %d model(s), judge=%s\n\n", len(selEvals), len(selModels), judge.Ref())
		events := make(chan runner.Event, 128)
		go drainEvents(events)

		res, err := runner.Run(context.Background(), runner.Options{
			Evals:       selEvals,
			Models:      selModels,
			Judge:       judge,
			Cfg:         cfg,
			AgentBudget: adapter.Budget{Timeout: flagTimeout},
			JudgeTO:     flagJudgeTO,
			Now:         time.Now(),
			Events:      events,
		})
		close(events)
		if err != nil {
			return err
		}
		renderLeaderboard(res)
		fmt.Printf("\nFull results: %s/run.json\n", res.Dir)
		return nil
	},
}

func init() {
	runCmd.Flags().DurationVar(&flagTimeout, "timeout", 20*time.Minute, "per-agent run timeout")
	runCmd.Flags().DurationVar(&flagJudgeTO, "judge-timeout", 5*time.Minute, "judge invocation timeout")
	runCmd.Flags().StringVar(&flagJudgeRef, "judge", "", "judge model ref (agent:model); skips the prompt")
}

func pick[T any](all []T, idxs []int) []T {
	out := make([]T, 0, len(idxs))
	for _, i := range idxs {
		out = append(out, all[i])
	}
	return out
}

func drainEvents(ch <-chan runner.Event) {
	for e := range ch {
		if e.Stage == runner.StageError {
			fmt.Printf("  ✗ %-24s %-22s %v\n", e.Eval, e.Model, e.Err)
			continue
		}
		fmt.Printf("  · %-24s %-22s %s\n", e.Eval, e.Model, e.Stage)
	}
}

var (
	hdr  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	good = lipgloss.NewStyle().Foreground(lipgloss.Color("84"))
)

func renderLeaderboard(res *runner.RunResult) {
	fmt.Println("\n" + hdr.Render("Leaderboard"))
	for rank, row := range res.Leaderboard {
		fmt.Printf("  %d. %-24s %s  (%d run(s))\n",
			rank+1, row.Model, good.Render(fmt.Sprintf("%.1f", row.Score)), row.Runs)
	}
	fmt.Println("\n" + hdr.Render("Per-eval breakdown"))
	for _, r := range res.Results {
		score := fmt.Sprintf("%.1f", r.Composite)
		if r.Err != "" {
			score = "ERR"
		}
		fmt.Printf("  %-24s %-22s %5s   judge=%.0f gate=%.2f\n",
			r.Eval, r.Model, score, r.JudgeOverall, r.GateFactor)
	}
}

var _ = score.Result{} // score types referenced via runner
