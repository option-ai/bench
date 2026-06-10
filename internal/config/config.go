// Package config owns the on-disk layout under ~/.config/bench and the two
// JSON files we persist: config.json (defaults, rubric weights, model registry)
// and auth.json (provider keys, written 0600).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Dir is the root config directory. Override with $BENCH_HOME for tests.
func Dir() string {
	if h := os.Getenv("BENCH_HOME"); h != "" {
		return h
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "bench")
}

func SnapshotsDir() string { return filepath.Join(Dir(), "snapshots") }
func CacheDir() string     { return filepath.Join(Dir(), "cache") }
func RunsDir() string      { return filepath.Join(Dir(), "runs") }
func configPath() string   { return filepath.Join(Dir(), "config.json") }
func authPath() string     { return filepath.Join(Dir(), "auth.json") }

// EnsureDirs creates the directory tree if missing. Cheap, idempotent.
func EnsureDirs() error {
	for _, d := range []string{Dir(), SnapshotsDir(), CacheDir(), RunsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// ReplayMode controls how a snapshot's prompts are fed to an agent.
type ReplayMode string

const (
	// ReplayOneShot collapses every user message into a single prompt (default).
	ReplayOneShot ReplayMode = "oneshot"
	// ReplaySequential replays each user message as its own turn, resuming the
	// agent session between turns.
	ReplaySequential ReplayMode = "sequential"
)

// GateWeights tunes how deterministic gates fold into the composite score.
// The composite is judge_overall * gate_factor, where gate_factor starts at 1.0
// and each failing gate applies its penalty. A build failure caps hard.
type GateWeights struct {
	// BuildFailCap is the multiplier applied when the build gate fails.
	BuildFailCap float64 `json:"build_fail_cap"`
	// LintPenalty is subtracted from gate_factor when lint fails.
	LintPenalty float64 `json:"lint_penalty"`
	// TestUseRatio, when true, scales gate_factor by the test pass ratio
	// instead of treating tests as binary.
	TestUseRatio bool `json:"test_use_ratio"`
}

// RubricWeights are the relative weights of the judge's sub-scores. They are
// normalized at scoring time, so they need not sum to 1.
type RubricWeights struct {
	TaskCompletion  float64 `json:"task_completion"`
	Correctness     float64 `json:"correctness"`
	FeedbackAdhere  float64 `json:"feedback_adherence"`
	ScopeDiscipline float64 `json:"scope_discipline"`
}

// Config is the persisted, versioned scoring + run configuration. Version is
// stamped into every run so historical numbers stay comparable.
type Config struct {
	Version       int           `json:"version"`
	DefaultJudge  string        `json:"default_judge"` // e.g. "claude-code:claude-opus-4-8"
	DefaultReplay ReplayMode    `json:"default_replay"`
	JudgeSamples  int           `json:"judge_samples"` // best-of-N median; 1 = single
	Concurrency   int           `json:"concurrency"`
	Rubric        RubricWeights `json:"rubric"`
	Gates         GateWeights   `json:"gates"`
	// Models optionally overrides an agent's selectable model list, keyed by
	// agent id (e.g. "claude-code"). Empty/absent => use the agent's built-ins.
	Models map[string][]string `json:"models,omitempty"`
}

// Default returns the baseline config used on first run.
func Default() Config {
	return Config{
		Version:       1,
		DefaultJudge:  "claude-code:claude-opus-4-8",
		DefaultReplay: ReplayOneShot,
		JudgeSamples:  1,
		Concurrency:   3,
		Rubric: RubricWeights{
			TaskCompletion:  0.40,
			Correctness:     0.30,
			FeedbackAdhere:  0.20,
			ScopeDiscipline: 0.10,
		},
		Gates: GateWeights{
			BuildFailCap: 0.30,
			LintPenalty:  0.10,
			TestUseRatio: false,
		},
	}
}

// Load reads config.json, falling back to defaults (and writing them) if absent.
func Load() (Config, error) {
	b, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		c := Default()
		return c, Save(c)
	}
	if err != nil {
		return Config{}, err
	}
	c := Default() // defaults fill any missing fields
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse config.json: %w", err)
	}
	return c, nil
}

// Save writes config.json pretty-printed.
func Save(c Config) error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(configPath(), b, 0o644)
}

// Auth maps a provider key (e.g. "openai", "anthropic") to an API key.
type Auth map[string]string

// LoadAuth reads auth.json, returning an empty map if absent.
func LoadAuth() (Auth, error) {
	b, err := os.ReadFile(authPath())
	if os.IsNotExist(err) {
		return Auth{}, nil
	}
	if err != nil {
		return nil, err
	}
	var a Auth
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("parse auth.json: %w", err)
	}
	return a, nil
}

// SaveAuth writes auth.json with 0600 perms (it holds secrets).
func SaveAuth(a Auth) error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(a, "", "  ")
	return os.WriteFile(authPath(), b, 0o600)
}
