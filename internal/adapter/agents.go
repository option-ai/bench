package adapter

import "context"

// Model lists are intentionally static and editable: bench treats them as the
// menu of selectable ids, not a source of truth about what each provider
// currently ships. Adjust per your access. Availability is binary-on-PATH.

func init() {
	Register(&claudeCode{})
	Register(&codex{})
	Register(&cursorAgent{})
	Register(&openCode{})
}

// ---- Claude Code -----------------------------------------------------------

type claudeCode struct{}

func (claudeCode) ID() string        { return "claude-code" }
func (claudeCode) Available() bool   { return onPath("claude") }
func (claudeCode) Models() []string {
	return []string{"claude-opus-4-8", "claude-sonnet-4-6", "claude-haiku-4-5"}
}

func (c *claudeCode) Run(ctx context.Context, dir string, turns []string, model string, b Budget) error {
	// First turn starts a fresh session; later turns resume it so sequential
	// replay keeps conversation memory.
	for i, t := range turns {
		args := []string{"-p", t, "--model", model, "--dangerously-skip-permissions"}
		if i > 0 {
			args = append(args, "--continue")
		}
		if _, err := run(ctx, dir, b, "claude", args...); err != nil {
			return err
		}
	}
	return nil
}

// ---- Codex CLI -------------------------------------------------------------

type codex struct{}

func (codex) ID() string      { return "codex" }
func (codex) Available() bool { return onPath("codex") }
func (codex) Models() []string {
	return []string{"gpt-5-codex", "gpt-5"}
}

func (c *codex) Run(ctx context.Context, dir string, turns []string, model string, b Budget) error {
	// codex exec is non-interactive. It has no cross-call session resume here,
	// so sequential turns run fresh against the (already-modified) tree.
	for _, t := range turns {
		args := []string{"exec", "--model", model, "--full-auto", t}
		if _, err := run(ctx, dir, b, "codex", args...); err != nil {
			return err
		}
	}
	return nil
}

// ---- cursor-agent ----------------------------------------------------------

type cursorAgent struct{}

func (cursorAgent) ID() string      { return "cursor-agent" }
func (cursorAgent) Available() bool { return onPath("cursor-agent") }
func (cursorAgent) Models() []string {
	return []string{"auto", "claude-sonnet-4-6", "gpt-5"}
}

func (c *cursorAgent) Run(ctx context.Context, dir string, turns []string, model string, b Budget) error {
	for _, t := range turns {
		args := []string{"-p", t, "--model", model, "--force"}
		if _, err := run(ctx, dir, b, "cursor-agent", args...); err != nil {
			return err
		}
	}
	return nil
}

// ---- opencode --------------------------------------------------------------

type openCode struct{}

func (openCode) ID() string      { return "opencode" }
func (openCode) Available() bool { return onPath("opencode") }
func (openCode) Models() []string {
	return []string{"anthropic/claude-opus-4-8", "openai/gpt-5"}
}

func (c *openCode) Run(ctx context.Context, dir string, turns []string, model string, b Budget) error {
	for _, t := range turns {
		args := []string{"run", "--model", model, t}
		if _, err := run(ctx, dir, b, "opencode", args...); err != nil {
			return err
		}
	}
	return nil
}
