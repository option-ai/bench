// Package runner orchestrates a bench run: clone each repo once, add a detached
// worktree per (eval x model) at the eval's commit, drive the agent, capture
// the diff, run deterministic gates, then hand the diff to a blind judge and
// fold everything into a composite score.
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/abdul/bench/internal/adapter"
	"github.com/abdul/bench/internal/config"
	"github.com/abdul/bench/internal/judge"
	"github.com/abdul/bench/internal/score"
	"github.com/abdul/bench/internal/snapshot"
)

// Stage marks where a job is in the pipeline (for the TUI status grid).
type Stage string

const (
	StageQueued Stage = "queued"
	StageClone  Stage = "clone"
	StageAgent  Stage = "agent"
	StageGates  Stage = "gates"
	StageJudge  Stage = "judge"
	StageDone   Stage = "done"
	StageError  Stage = "error"
)

// Event is a progress update for one job.
type Event struct {
	Eval  string
	Model string
	Stage Stage
	Err   error
}

// Options configures a run.
type Options struct {
	Evals       []*snapshot.Snapshot
	Models      []adapter.ModelRef
	Judge       adapter.ModelRef
	Cfg         config.Config
	AgentBudget adapter.Budget
	JudgeTO     time.Duration
	Now         time.Time // injected so the package stays deterministic/testable
	Events      chan<- Event // optional; nil to disable progress
}

// RunResult is the persisted outcome of a whole run.
type RunResult struct {
	ID          string         `json:"id"`
	StartedAt   string         `json:"started_at"`
	ConfigVer   int            `json:"config_version"`
	Judge       string         `json:"judge"`
	Results     []score.Result `json:"results"`
	Leaderboard []score.LeaderRow `json:"leaderboard"`
	Dir         string         `json:"-"`
}

func emit(ch chan<- Event, e Event) {
	if ch != nil {
		ch <- e
	}
}

// Run executes the bench and returns the scored, persisted result.
func Run(ctx context.Context, o Options) (*RunResult, error) {
	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}
	runID := o.Now.Format("2006-01-02T15-04-05")
	runDir := filepath.Join(config.RunsDir(), runID)
	work := filepath.Join(runDir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}

	// Clone each distinct repo once into the run's cache, reused across jobs.
	clones := map[string]string{}
	var cloneMu sync.Mutex
	cloneRepo := func(repo string) (string, error) {
		cloneMu.Lock()
		defer cloneMu.Unlock()
		if p, ok := clones[repo]; ok {
			return p, nil
		}
		dest := filepath.Join(config.CacheDir(), snapshot.Slug(repo))
		if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
			if err := gitClone(ctx, repo, dest); err != nil {
				return "", err
			}
		} else {
			_ = gitFetch(ctx, dest) // refresh existing cache, best-effort
		}
		clones[repo] = dest
		return dest, nil
	}

	// Build the job matrix.
	type job struct {
		eval  *snapshot.Snapshot
		model adapter.ModelRef
	}
	var jobs []job
	for _, e := range o.Evals {
		for _, m := range o.Models {
			jobs = append(jobs, job{e, m})
		}
	}

	conc := o.Cfg.Concurrency
	if conc < 1 {
		conc = 1
	}
	sem := make(chan struct{}, conc)
	results := make([]score.Result, len(jobs))
	var wg sync.WaitGroup

	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = runJob(ctx, o, j.eval, j.model, work, cloneRepo)
		}(i, j)
	}
	wg.Wait()

	res := &RunResult{
		ID:          runID,
		StartedAt:   o.Now.Format(time.RFC3339),
		ConfigVer:   o.Cfg.Version,
		Judge:       o.Judge.Ref(),
		Results:     results,
		Leaderboard: score.Leaderboard(results),
		Dir:         runDir,
	}
	if err := persist(runDir, res); err != nil {
		return res, err
	}
	return res, nil
}

func runJob(ctx context.Context, o Options, e *snapshot.Snapshot, m adapter.ModelRef, work string, cloneRepo func(string) (string, error)) score.Result {
	r := score.Result{Eval: e.Title, Model: m.Ref()}
	fail := func(stage Stage, err error) score.Result {
		emit(o.Events, Event{e.Title, m.Ref(), StageError, err})
		r.Err = fmt.Sprintf("%s: %v", stage, err)
		return r
	}

	// 1. set up the workspace: clone+worktree for repo-backed evals, or a fresh
	// git-initialised scratch dir for evals captured without a repo.
	emit(o.Events, Event{e.Title, m.Ref(), StageClone, nil})
	wt := filepath.Join(work, snapshot.Slug(e.Title)+"__"+snapshot.Slug(m.Ref()))
	if e.IsScratch() {
		if err := scratchWorkspace(ctx, wt); err != nil {
			return fail(StageClone, err)
		}
	} else {
		cache, err := cloneRepo(e.Repo)
		if err != nil {
			return fail(StageClone, err)
		}
		if err := gitWorktreeAdd(ctx, cache, wt, e.Commit); err != nil {
			return fail(StageClone, err)
		}
	}

	// 2. drive the agent. Collapse prompts for oneshot replay. Capture its final
	// written output so text-answer evals are gradable even with no file changes.
	emit(o.Events, Event{e.Title, m.Ref(), StageAgent, nil})
	ag := adapter.Get(m.Agent)
	if ag == nil || !ag.Available() {
		return fail(StageAgent, fmt.Errorf("agent %q unavailable", m.Agent))
	}
	turns := buildTurns(e)
	output, err := ag.Run(ctx, wt, turns, m.Model, o.AgentBudget)
	if err != nil {
		return fail(StageAgent, err)
	}

	// 3. capture the diff (including new files; empty for pure text answers).
	diff, err := gitCaptureDiff(ctx, wt)
	if err != nil {
		return fail(StageAgent, err)
	}

	// 4. deterministic gates.
	emit(o.Events, Event{e.Title, m.Ref(), StageGates, nil})
	gates := runGates(ctx, wt, e.Gates, o.AgentBudget.Timeout)

	// 5. blind judge.
	emit(o.Events, Event{e.Title, m.Ref(), StageJudge, nil})
	sub, rationale, err := judge.Judge(ctx, o.Judge, judge.Input{
		Task: e.Prompts, Feedback: e.Feedback, Diff: diff, Output: output,
	}, o.Cfg.JudgeSamples, o.JudgeTO)
	if err != nil {
		return fail(StageJudge, err)
	}

	out := score.Compute(e.Title, m.Ref(), sub, gates, o.Cfg)
	out.Rationale = rationale
	emit(o.Events, Event{e.Title, m.Ref(), StageDone, nil})
	return out
}

// buildTurns collapses prompts into a single turn for oneshot replay, or
// returns them as-is for sequential.
func buildTurns(e *snapshot.Snapshot) []string {
	if e.Replay == config.ReplaySequential {
		return e.Prompts
	}
	var b strings.Builder
	b.WriteString("Complete the following request(s) in this repository.\n\n")
	for i, p := range e.Prompts {
		fmt.Fprintf(&b, "--- Message %d ---\n%s\n\n", i+1, p)
	}
	return []string{strings.TrimSpace(b.String())}
}

// runGates executes the snapshot's build/test/lint commands in the worktree.
func runGates(ctx context.Context, dir string, g snapshot.Gates, timeout time.Duration) score.GateResult {
	var res score.GateResult
	if g.Build != "" {
		ok := shellOK(ctx, dir, g.Build, timeout)
		res.Build = score.GateOutcome{Ran: true, Passed: ok}
	}
	if g.Test != "" {
		ok := shellOK(ctx, dir, g.Test, timeout)
		ratio := 0.0
		if ok {
			ratio = 1.0
		}
		res.Test = score.TestOutcome{Ran: true, Passed: ok, Ratio: ratio}
	}
	if g.Lint != "" {
		ok := shellOK(ctx, dir, g.Lint, timeout)
		res.Lint = score.GateOutcome{Ran: true, Passed: ok}
	}
	return res
}

// shellOK runs a command string via the shell and reports zero exit.
func shellOK(ctx context.Context, dir, command string, timeout time.Duration) bool {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	return cmd.Run() == nil
}
