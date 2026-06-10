# bench

A personal benchmark for coding agents, seeded from your **real** sessions.

Capture the prompts from a Claude Code conversation as an *eval* (with the repo
+ commit you were on). Later, replay those prompts against any installed coding
agent on that exact repo state, and a **blind judge** scores each diff into a
single composite number so you can compare models head-to-head.

```
/add-to-bench   capture the current session as an eval   (Claude Code skill)
bench setup     guided setup: install skill, agent logins, pick default judge
bench run       pick evals × models, run, score (judge from config or --judge)
bench run --evals a,b --models claude-code:claude-fable-5   # non-interactive
bench list      list your evals
bench results   all-time leaderboard, run history, detail, compare two runs
bench models    show detected agents + the model ids bench will offer
bench install   non-interactive: install skill + config only
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

## Auth

Every supported agent authenticates with **its own login** — bench stores no
API keys at all. `bench setup` walks you through it and can launch each login
inline:

| agent        | login                                         |
|--------------|-----------------------------------------------|
| claude-code  | run `claude`, then `/login` (or `claude setup-token`) |
| codex        | `codex login` (your ChatGPT/Codex login, not an API key) |
| cursor-agent | `cursor-agent login`                          |
| opencode     | `opencode auth login`                         |

## Scoring (the single number)

Each `(eval × model)` run collapses to a composite `0–100`:

```
composite = judge_overall × gate_factor
```

- `judge_overall` — rubric-weighted mean of four 0–100 sub-scores:
  task completion (0.40), correctness (0.30), feedback adherence (0.20),
  scope discipline (0.10). Weights live in `~/.config/bench/config.json`.
- `gate_factor` — starts at 1.0; **build failure caps it at 0.30**, test
  failure ×0.5 (`test_fail_factor`), lint failure −0.10. Clamped to `[0,1]`.
  So a pretty diff that doesn't compile can't beat an ugly one that does.

A model's leaderboard number is the mean composite across the evals it ran.
The config version is stamped into every run so numbers stay comparable.
`bench results` aggregates every persisted run into an all-time leaderboard,
shows any past run in full (scores, rationales, artifact paths), and
`bench results compare <a> <b>` diffs two runs model-by-model.

## Rate limits

The agent CLIs don't expose RPM/TPM quotas, so bench can't query limits — it
defends instead: at most `per_agent_concurrency` (default 2) concurrent jobs
per agent CLI, and failures that look like rate-limiting (429, quota,
overloaded…) get the workspace reset and the job retried with backoff
(30s/90s/180s, `rate_limit_retries` times). A job that stays rate-limited is
reported as `rate-limited (retries exhausted)` rather than a silent 0.

## Replay modes

- `oneshot` (default) — all user messages collapsed into a single prompt.
- `sequential` — each message replayed as its own turn (session resumed for
  agents that support it).

Per-eval `replay:` in the snapshot wins; evals without one use `default_replay`
from config; absent both, oneshot. The `--timeout` flag bounds each agent job's
whole run (all turns).

## Layout

```
~/.config/bench/
  snapshots/*.md       evals
  config.json          scoring weights, defaults, default judge, model overrides
  cache/               cloned repos + model-list caches, reused across runs
  runs/<ts>/run.json   scored results + leaderboard
  runs/<ts>/jobs/      per-job artifacts: diff.patch, output.txt
```

Working trees are created per job and removed once their artifacts are saved —
runs don't accumulate checkouts.

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

## Judge integrity

The judge sees the task, the rubric, and the work (diff truncated at 80KB,
written answer at 40KB) — never the model identity or the agent transcript. It
runs in an empty directory so a tool-capable judge can't inspect any workspace,
and agent answers are extracted cleanly (codex via `--output-last-message`,
ANSI stripped everywhere) so CLI output formats don't fingerprint the tool.

## Status

Working: capture format (repo-backed + scratch), scoring engine (tested), agent
detection, run orchestration (clone/worktree or scratch workspace, diff, gates,
artifact persistence, cleanup, ctrl+c cancellation), blind judge, leaderboard,
non-interactive runs, guided setup.

Model lists: codex and opencode are read from the tools themselves; claude-code
and cursor-agent are editable defaults. Override any of them via the "models"
map in config.json. Test gates are binary (pass/fail); per-framework pass-ratio
parsing is a future addition.

## Build

```
go build -o bin/bench .
go test ./...
```
