# bench

A personal benchmark for coding agents, seeded from your **real** sessions.

Capture the prompts from a Claude Code conversation as an *eval* (with the repo
+ commit you were on). Later, replay those prompts against any installed coding
agent on that exact repo state, and a **blind judge** scores each diff into a
single composite number so you can compare models head-to-head.

```
/add-to-bench           capture the current session as an eval   (Claude Code skill)
bench run               pick evals × models × judge, run, score
bench list              list your evals
bench install           install the skill globally + set up config
bench auth set <p> <k>  store a provider API key
```

## Concepts

- **eval** — one captured task: prompts + optional feedback rubric, optionally
  anchored to a repo + commit. One markdown file under
  `~/.config/bench/snapshots/`. Two flavours:
  - **repo-backed** — replays the prompts against an exact repo state; the judge
    scores the diff.
  - **scratch** — no repo (sessions captured in Claude Desktop, ChatGPT desktop,
    Cowork, or from-scratch tasks). The agent runs in a fresh empty workspace
    and the judge scores whatever it produces — created files and/or its written
    answer.
- **bench** — the set of evals you select for a run.
- **judge** — a model that grades a diff *blind*: it sees the task and the
  feedback rubric, never the model identity or the agent's reasoning.

## Scoring (the single number)

Each `(eval × model)` run collapses to a composite `0–100`:

```
composite = judge_overall × gate_factor
```

- `judge_overall` — rubric-weighted mean of four 0–100 sub-scores:
  task completion (0.40), correctness (0.30), feedback adherence (0.20),
  scope discipline (0.10). Weights live in `~/.config/bench/config.json`.
- `gate_factor` — starts at 1.0; **build failure caps it at 0.30**, test
  failure ×0.5 (or by pass-ratio if enabled), lint failure −0.10. Clamped to
  `[0,1]`. So a pretty diff that doesn't compile can't beat an ugly one that does.

A model's leaderboard number is the mean composite across the evals it ran.
The config version is stamped into every run so numbers stay comparable.

## Replay modes

- `oneshot` (default) — all user messages collapsed into a single prompt.
- `sequential` — each message replayed as its own turn (session resumed for
  agents that support it). Set per-eval via `replay:` in the snapshot, or
  globally via `default_replay` in config.

## Layout

```
~/.config/bench/
  snapshots/*.md     evals
  config.json        scoring weights, defaults, judge
  auth.json          provider keys (0600)
  cache/             cloned repos, reused across runs
  runs/<ts>/run.json scored results + leaderboard
```

## Architecture

```
cmd/                 cobra CLI: run, list, auth, install
internal/
  config/            on-disk layout, config.json + auth.json, scoring weights
  snapshot/          parse/write eval markdown (YAML frontmatter + prompt blocks)
  adapter/           Agent interface + claude-code/codex/cursor-agent/opencode
  judge/             blind grader: builds the prompt, invokes a model, parses JSON
  score/             composite scoring + leaderboard (unit-tested)
  runner/            orchestration: clone → worktree → agent → diff → gates → judge
  tui/               Bubble Tea selection screens
  skill/             embedded /add-to-bench SKILL.md
```

## Status

Working: capture format (repo-backed + scratch), scoring engine (tested), agent
detection, run orchestration (clone/worktree or scratch workspace, diff, gates),
blind judge (grades a diff and/or a written answer), leaderboard, install.

Adapter command flags (`codex exec`, `cursor-agent -p`, `opencode run`) and the
static model lists are best-effort starting points — tune `internal/adapter/agents.go`
to your actual CLI versions and model access. Test gates are currently binary
(pass/fail); per-framework pass-ratio parsing is a future addition.

## Build

```
go build -o bin/bench .
go test ./...
```
