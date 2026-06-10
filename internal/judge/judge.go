// Package judge runs a model as a blind grader of a diff. The judge is given
// the task (the user prompts), the optional feedback rubric, and the diff. It
// is NOT told which agent or model produced the diff, and never sees the
// agent's reasoning. That blindness is the point.
package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/abdul/bench/internal/adapter"
	"github.com/abdul/bench/internal/score"
)

// Input is everything the judge is allowed to see.
type Input struct {
	Task     []string // the user prompts (the task)
	Feedback string   // optional rubric note from the snapshot
	Diff     string   // unified diff the agent produced
}

type rawScore struct {
	TaskCompletion    float64 `json:"task_completion"`
	Correctness       float64 `json:"correctness"`
	FeedbackAdherence float64 `json:"feedback_adherence"`
	ScopeDiscipline   float64 `json:"scope_discipline"`
	Rationale         string  `json:"rationale"`
}

// Judge grades the diff, sampling `samples` times and taking the per-dimension
// median for stability. Returns sub-scores and the rationale from the median run.
func Judge(ctx context.Context, ref adapter.ModelRef, in Input, samples int, timeout time.Duration) (score.Subscores, string, error) {
	if samples < 1 {
		samples = 1
	}
	prompt := buildPrompt(in)

	var (
		tc, co, fa, sd []float64
		rationale      string
	)
	for i := 0; i < samples; i++ {
		out, err := generate(ctx, ref, prompt, timeout)
		if err != nil {
			return score.Subscores{}, "", fmt.Errorf("judge invoke: %w", err)
		}
		rs, err := parse(out)
		if err != nil {
			return score.Subscores{}, "", fmt.Errorf("judge output: %w", err)
		}
		tc = append(tc, rs.TaskCompletion)
		co = append(co, rs.Correctness)
		fa = append(fa, rs.FeedbackAdherence)
		sd = append(sd, rs.ScopeDiscipline)
		rationale = rs.Rationale
	}
	return score.Subscores{
		TaskCompletion:    score.Median(tc),
		Correctness:       score.Median(co),
		FeedbackAdherence: score.Median(fa),
		ScopeDiscipline:   score.Median(sd),
	}, rationale, nil
}

func buildPrompt(in Input) string {
	var b strings.Builder
	b.WriteString(`You are a strict, impartial code-review judge in an LLM benchmark.
You are shown a TASK and a DIFF that some agent produced to accomplish it.
You do NOT know which model or tool produced the diff. Judge only what you see.

Score four dimensions from 0 to 100:
- task_completion: did the diff actually accomplish what the task asked?
- correctness: is the code correct, idiomatic, and free of bugs?
- feedback_adherence: does it satisfy the reviewer's feedback note? If no note is given, score this 100.
- scope_discipline: did it stay on-task without unrelated or destructive changes?

Respond with ONLY a JSON object, no prose around it:
{"task_completion":<n>,"correctness":<n>,"feedback_adherence":<n>,"scope_discipline":<n>,"rationale":"<one paragraph>"}

== TASK ==
`)
	for i, t := range in.Task {
		fmt.Fprintf(&b, "%d. %s\n", i+1, t)
	}
	b.WriteString("\n== REVIEWER FEEDBACK NOTE ==\n")
	if strings.TrimSpace(in.Feedback) == "" {
		b.WriteString("(none provided — score feedback_adherence 100)\n")
	} else {
		b.WriteString(in.Feedback + "\n")
	}
	b.WriteString("\n== DIFF ==\n")
	if strings.TrimSpace(in.Diff) == "" {
		b.WriteString("(empty diff — the agent changed nothing)\n")
	} else {
		b.WriteString(in.Diff + "\n")
	}
	return b.String()
}

var jsonRe = regexp.MustCompile(`(?s)\{.*\}`)

func parse(out string) (rawScore, error) {
	m := jsonRe.FindString(out)
	if m == "" {
		return rawScore{}, fmt.Errorf("no JSON object in judge output")
	}
	var rs rawScore
	if err := json.Unmarshal([]byte(m), &rs); err != nil {
		return rawScore{}, err
	}
	return rs, nil
}

// generate invokes a model purely as a text generator (no file access) and
// returns stdout. Command shape is per-agent; the judge only needs the text.
func generate(ctx context.Context, ref adapter.ModelRef, prompt string, timeout time.Duration) (string, error) {
	var name string
	var args []string
	switch ref.Agent {
	case "claude-code":
		name, args = "claude", []string{"-p", prompt, "--model", ref.Model}
	case "codex":
		name, args = "codex", []string{"exec", "--model", ref.Model, prompt}
	case "cursor-agent":
		name, args = "cursor-agent", []string{"-p", prompt, "--model", ref.Model}
	case "opencode":
		name, args = "opencode", []string{"run", "--model", ref.Model, prompt}
	default:
		return "", fmt.Errorf("unknown judge agent %q", ref.Agent)
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}
