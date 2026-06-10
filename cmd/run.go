package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	flagEvals    []string
	flagModels   []string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Select evals + models, run them, and score into a leaderboard",
	Long: `Interactively pick evals and models, or pass --evals/--models/--judge to skip
the pickers entirely (useful for scripting). The judge defaults to the one
chosen during ` + "`bench setup`" + ` (default_judge in config.json).`,
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

		// 1. evals: --evals flag, or picker
		var selEvals []*snapshot.Snapshot
		if len(flagEvals) > 0 {
			selEvals, err = evalsByName(snaps, flagEvals)
			if err != nil {
				return err
			}
		} else {
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
			selEvals = pick(snaps, ei)
		}

		// 2. models: --models flag, or picker
		var selModels []adapter.ModelRef
		if len(flagModels) > 0 {
			for _, m := range flagModels {
				ref, err := adapter.ParseRef(m)
				if err != nil {
					return err
				}
				selModels = append(selModels, ref)
			}
		} else {
			modelItems := make([]tui.Item, len(models))
			for i, m := range models {
				modelItems[i] = tui.Item{Label: m.Ref(), Desc: m.Agent}
			}
			mi, err := tui.PickMany("Select models", modelItems)
			if err != nil {
				return err
			}
			selModels = pick(models, mi)
		}

		// 3. judge: --judge flag > config default_judge > picker
		var judge adapter.ModelRef
		switch {
		case flagJudgeRef != "":
			judge, err = adapter.ParseRef(flagJudgeRef)
			if err != nil {
				return err
			}
		case cfg.DefaultJudge != "":
			judge, err = adapter.ParseRef(cfg.DefaultJudge)
			if err != nil {
				return fmt.Errorf("config default_judge: %w", err)
			}
		default:
			modelItems := make([]tui.Item, len(models))
			for i, m := range models {
				modelItems[i] = tui.Item{Label: m.Ref(), Desc: m.Agent}
			}
			ji, err := tui.PickOne("Select the judge (blind grader — sees only the diff + rubric)", modelItems)
			if err != nil {
				return err
			}
			judge = models[ji]
		}

		// 4. run with a live progress view and a cancellable context: if the
		// user quits the view, the agents must die too (they cost money).
		fmt.Printf("\nRunning %d eval(s) × %d model(s) · judge %s %s\n\n",
			len(selEvals), len(selModels), judge.Ref(), dimNote(flagJudgeRef, cfg))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		events := make(chan runner.Event, 256)
		resCh := make(chan *runner.RunResult, 1)
		errCh := make(chan error, 1)
		go func() {
			r, err := runner.Run(ctx, runner.Options{
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
		interrupted := false
		if fi, _ := os.Stdout.Stat(); fi != nil && fi.Mode()&os.ModeCharDevice != 0 {
			var uiErr error
			interrupted, uiErr = tui.RunProgress(evalLabels, modelLabels, events)
			if uiErr != nil {
				// The view failing must not kill paid agent runs; fall back to
				// plain progress on whatever events remain.
				fmt.Printf("(live view unavailable: %v)\n", uiErr)
				tui.RunProgressPlain(evalLabels, modelLabels, events)
			}
		} else {
			tui.RunProgressPlain(evalLabels, modelLabels, events)
		}
		if interrupted {
			fmt.Println("\nInterrupted — stopping agents…")
			cancel()
		}
		// Drain any events still buffered/being emitted so the runner can
		// finish (emit blocks once the buffer fills if no one reads).
		go func() {
			for range events {
			}
		}()

		select {
		case err := <-errCh:
			return err
		case res := <-resCh:
			if interrupted {
				fmt.Println("Run cancelled; partial results below.")
			}
			fmt.Print(tui.RenderResults(res))
			fmt.Printf("\nFull results: %s/run.json\n", res.Dir)
		}
		return nil
	},
}

func dimNote(judgeFlag string, cfg config.Config) string {
	if judgeFlag != "" {
		return "(--judge)"
	}
	if cfg.DefaultJudge != "" {
		return "(config default; override with --judge)"
	}
	return ""
}

// evalsByName resolves --evals values against titles and slugs.
func evalsByName(snaps []*snapshot.Snapshot, names []string) ([]*snapshot.Snapshot, error) {
	var out []*snapshot.Snapshot
	for _, n := range names {
		found := false
		for _, s := range snaps {
			if s.Title == n || snapshot.Slug(s.Title) == snapshot.Slug(n) {
				out = append(out, s)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("no eval named %q (have: %s)", n, evalNames(snaps))
		}
	}
	return out, nil
}

func evalNames(snaps []*snapshot.Snapshot) string {
	var names []string
	for _, s := range snaps {
		names = append(names, s.Title)
	}
	return strings.Join(names, ", ")
}

func init() {
	runCmd.Flags().DurationVar(&flagTimeout, "timeout", 20*time.Minute, "whole-run timeout per agent job (all turns)")
	runCmd.Flags().DurationVar(&flagJudgeTO, "judge-timeout", 5*time.Minute, "judge invocation timeout")
	runCmd.Flags().StringVar(&flagJudgeRef, "judge", "", "judge model ref (agent:model); overrides config default_judge")
	runCmd.Flags().StringSliceVar(&flagEvals, "evals", nil, "eval titles to run (skips the picker)")
	runCmd.Flags().StringSliceVar(&flagModels, "models", nil, "model refs to run, agent:model (skips the picker)")
}

func pick[T any](all []T, idxs []int) []T {
	out := make([]T, 0, len(idxs))
	for _, i := range idxs {
		out = append(out, all[i])
	}
	return out
}
