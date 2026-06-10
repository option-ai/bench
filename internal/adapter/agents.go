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

func (claudeCode) ID() string      { return "claude-code" }
func (claudeCode) Available() bool { return onPath("claude") }
func (claudeCode) Models() []string {
	return []string{"claude-opus-4-8", "claude-sonnet-4-6", "claude-haiku-4-5"}
}
func (claudeCode) Auth() AuthInfo {
	return AuthInfo{Note: "Uses your Claude Code login/subscription. Log in by running `claude` and using /login (or `claude setup-token`)."}
}

func (c *claudeCode) Run(ctx context.Context, dir string, turns []string, model string, b Budget) (string, error) {
	// First turn starts a fresh session; later turns resume it so sequential
	// replay keeps conversation memory.
	var last string
	for i, t := range turns {
		args := []string{"-p", t, "--model", model, "--dangerously-skip-permissions"}
		if i > 0 {
			args = append(args, "--continue")
		}
		out, err := run(ctx, dir, b, "claude", args...)
		last = string(out)
		if err != nil {
			return last, err
		}
	}
	return last, nil
}

// ---- Codex CLI -------------------------------------------------------------

type codex struct{}

func (codex) ID() string      { return "codex" }
func (codex) Available() bool { return onPath("codex") }
func (codex) Models() []string {
	return []string{"gpt-5-codex", "gpt-5"}
}
func (codex) Auth() AuthInfo {
	return AuthInfo{LoginCmd: "codex login", Note: "Uses your ChatGPT/Codex login — not an API key."}
}

func (c *codex) Run(ctx context.Context, dir string, turns []string, model string, b Budget) (string, error) {
	// codex exec is non-interactive. It has no cross-call session resume here,
	// so sequential turns run fresh against the (already-modified) tree.
	var last string
	for _, t := range turns {
		out, err := run(ctx, dir, b, "codex", "exec", "--model", model, "--full-auto", t)
		last = string(out)
		if err != nil {
			return last, err
		}
	}
	return last, nil
}

// ---- cursor-agent ----------------------------------------------------------

type cursorAgent struct{}

func (cursorAgent) ID() string      { return "cursor-agent" }
func (cursorAgent) Available() bool { return onPath("cursor-agent") }
func (cursorAgent) Models() []string {
	return []string{"auto", "claude-sonnet-4-6", "gpt-5"}
}
func (cursorAgent) Auth() AuthInfo {
	return AuthInfo{LoginCmd: "cursor-agent login", Note: "Uses your Cursor login."}
}

func (c *cursorAgent) Run(ctx context.Context, dir string, turns []string, model string, b Budget) (string, error) {
	var last string
	for _, t := range turns {
		out, err := run(ctx, dir, b, "cursor-agent", "-p", t, "--model", model, "--force")
		last = string(out)
		if err != nil {
			return last, err
		}
	}
	return last, nil
}

// ---- opencode --------------------------------------------------------------

type openCode struct{}

func (openCode) ID() string      { return "opencode" }
func (openCode) Available() bool { return onPath("opencode") }
func (openCode) Models() []string {
	return []string{"anthropic/claude-opus-4-8", "openai/gpt-5"}
}
func (openCode) Auth() AuthInfo {
	return AuthInfo{LoginCmd: "opencode auth login", Note: "Configure providers via opencode's own auth."}
}

func (c *openCode) Run(ctx context.Context, dir string, turns []string, model string, b Budget) (string, error) {
	var last string
	for _, t := range turns {
		out, err := run(ctx, dir, b, "opencode", "run", "--model", model, t)
		last = string(out)
		if err != nil {
			return last, err
		}
	}
	return last, nil
}
