---
name: add-to-bench
description: Capture the current Claude Code conversation as a bench eval — grabs all user prompts plus the repo and commit you're on, and writes a snapshot file that the `bench` CLI can later replay against other models. Use when the user says "add this to my bench", "/add-to-bench", "snapshot this for evals", or wants to turn the current session into a benchmark case.
---

# add-to-bench

Capture the current session as a **bench eval**: a markdown snapshot of the
user prompts + repo state that the `bench` CLI replays against coding models and
scores with a blind judge.

## Arguments (both optional)

- `title` — identifier and filename for the eval. If omitted, infer a short
  kebab-case title from what the session was about.
- `feedback` — a note that becomes the judge's rubric (e.g. "should debounce the
  DoH lookups, not add a global timeout"). If omitted, leave it empty.

Parse these from the user's invocation, e.g. `/add-to-bench title=fix-rdap feedback="..."`.

## Steps

1. **Collect the user prompts.** Gather every *user* message in the current
   conversation, in order — only genuine user turns, not tool results, system
   reminders, or your own messages. Preserve their full text verbatim.

2. **Capture repo state.** In the working directory, run:
   - `git remote get-url origin` → normalize to `github.com/owner/name` form
     (strip protocol, trailing `.git`).
   - `git rev-parse HEAD` → the commit.
   If the tree is dirty, warn the user that uncommitted changes won't be part of
   the snapshot (bench checks out the commit cleanly).

3. **Detect gate commands.** Sniff the repo for build/test/lint commands and
   fill what you find (leave unknown ones empty):
   - Node/Bun: `package.json` scripts → `build`, `test`, `lint`.
   - Go: `go build ./...`, `go test ./...`, `go vet ./...`.
   - Rust: `cargo build`, `cargo test`, `cargo clippy`.
   - Python: `pytest`, `ruff check .`, etc.

4. **Write the snapshot file** to
   `~/.config/bench/snapshots/<title-slug>.md` (create the directory if needed),
   in EXACTLY this format so the `bench` CLI can parse it:

   ```markdown
   ---
   title: <title>
   repo: github.com/owner/name
   commit: <full-sha>
   created: <YYYY-MM-DD>
   feedback: <feedback or omit the line if none>
   replay: oneshot
   gates:
       build: <cmd or omit>
       test: <cmd or omit>
       lint: <cmd or omit>
   ---

   ## Prompts

   <!-- prompt -->
   <first user prompt verbatim>

   <!-- prompt -->
   <second user prompt verbatim>
   ```

   - Each prompt block is preceded by a literal `<!-- prompt -->` line. This
     delimiter is how bench splits prompts, so include it before every prompt.
   - `replay: oneshot` is the default (all prompts collapsed into one). Use
     `replay: sequential` only if the user asks to preserve turn-by-turn replay.

5. **Confirm** to the user: print the path written, the title, repo@commit, the
   number of prompts captured, and the detected gates. Tell them they can run it
   with `bench run`.

## Notes

- Never invent prompts or paraphrase — capture them exactly.
- The eval is self-contained: prompts + repo + commit + feedback rubric. There
  is no reference solution; the judge scores against the feedback note and the
  deterministic gates.
