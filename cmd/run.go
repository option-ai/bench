package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/abdul/bench/internal/adapter"
	"github.com/abdul/bench/internal/config"
	"github.com/abdul/bench/internal/runner"
	"github.com/abdul/bench/internal/snapshot"
	"github.com/abdul/bench/internal/tui"
	"github.com/spf13/cobra"
)

var (
	flagTimeout  time.Duration
	flagJudgeTO  time.Duration
	flagJudgeRef string
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
		models := adapter.AvailableModelsWith(cfg.Models)
		if len(models) == 0 {
			fmt.Println("No coding agents found on PATH (claude, codex, cursor-agent, opencode).")
			return nil
		}

		// 1. pick evals
		evalItems := make([]tui.Item, len(snaps))
		for i, s := range snaps {
			anchor := "scratch"
			if !s.IsScratch() {
				anchor = fmt.Sprintf("%s@%.8s", s.Repo, s.Commit)
			}
			evalItems[i] = tui.Item{Label: s.Title, Desc: anchor}
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

		// 4. run with a live, aligned progress view
		fmt.Printf("\nRunning %d eval(s) × %d model(s) · judge %s\n\n", len(selEvals), len(selModels), judge.Ref())

		events := make(chan runner.Event, 256)
		resCh := make(chan *runner.RunResult, 1)
		errCh := make(chan error, 1)
		go func() {
			r, err := runner.Run(context.Background(), runner.Options{
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
				errCh <- err
			} else {
				resCh <- r
			}
		}()

		evalLabels := make([]string, len(selEvals))
		for i, e := range selEvals {
			evalLabels[i] = e.Title
		}
		modelLabels := make([]string, len(selModels))
		for i, m := range selModels {
			modelLabels[i] = m.Ref()
		}
		if err := tui.RunProgress(evalLabels, modelLabels, events); err != nil {
			return err
		}

		select {
		case err := <-errCh:
			return err
		case res := <-resCh:
			fmt.Print(tui.RenderResults(res))
			fmt.Printf("\nFull results: %s/run.json\n", res.Dir)
		}
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
