---
name: add-to-bench
description: Capture the current conversation as a bench eval — grabs all user prompts (and, if you're in a git repo, the repo + commit) and writes a snapshot the `bench` CLI can replay against other models. Works with or without a repo, so it's usable from coding sessions and from repo-less environments alike. Use when the user says "add this to my bench", "/add-to-bench", "snapshot this for evals", or wants to turn the current session into a benchmark case.
argument-hint: "[title] [feedback…]"
arguments: [title, feedback]
allowed-tools: Read Write Bash(git *) Bash(mkdir *)
---

# add-to-bench

Capture the current session as a **bench eval**: a markdown snapshot of the user
prompts (plus repo state when available) that the `bench` CLI replays against
coding models and scores with a blind judge.

An eval comes in two flavours, chosen automatically:

- **repo-backed** — you're inside a git repo. Capture the repo + commit so bench
  replays the prompts against that exact code state and judges the diff.
- **scratch** — no git repo (e.g. a from-scratch task, or a session in Claude
  Desktop / ChatGPT desktop / Cowork). Capture prompts only; bench runs the
  agent in a fresh empty workspace and judges whatever it produces (created
  files and/or its written answer).

## Arguments (both optional)

The raw invocation arguments are: `$ARGUMENTS`

- **title** — the FIRST whitespace-delimited token of the arguments. If there
  are no arguments, infer a short kebab-case title from what the session was
  about.
- **feedback** — EVERYTHING after the first token, as one string (feedback is
  normally a sentence; do not take just one word). If nothing follows the
  title, leave the frontmatter line out.

If the user instead wrote `title=... feedback="..."` or described them in
prose, honor that.

## Steps

1. **Collect the user prompts.** Gather every *user* message in the current
   conversation, in order — only genuine user turns, not tool results, system
   reminders, or your own messages. Preserve their full text verbatim, with two
   exclusions:
   - the `/add-to-bench` invocation itself (it is bookkeeping, not part of the
     task being benchmarked), and
   - any other slash-command invocations (`/foo ...`) that are harness
     commands rather than task content.
   If a prompt happens to contain the literal line `<!-- prompt -->`, indent it
   by two spaces inside the captured block so bench's splitter is not confused.

2. **Determine the anchor.** Check whether the working directory is a git repo
   (`git rev-parse --is-inside-work-tree`). 
   - **If yes (repo-backed):**
     - `git remote get-url origin` → normalize to `github.com/owner/name`
       (strip protocol, trailing `.git`).
     - `git rev-parse HEAD` → the commit.
     - If the tree is dirty, warn that uncommitted changes won't be captured
       (bench checks out the commit cleanly).
     - Detect gate commands and fill what you find (leave unknown ones empty):
       Node/Bun `package.json` scripts; Go `go build ./...` / `go test ./...` /
       `go vet ./...`; Rust `cargo build|test|clippy`; Python `pytest` / `ruff`.
   - **If no (scratch):** skip repo, commit, and gates entirely. Do not invent
     them. The eval will run in an empty workspace.

3. **Write the snapshot file** to `~/.config/bench/snapshots/<title-slug>.md`
   (create the directory if needed), in EXACTLY this format so `bench` can parse
   it. **Omit `repo`, `commit`, and the `gates` block for scratch evals.**

   Repo-backed:
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

   Scratch (no repo):
   ```markdown
   ---
   title: <title>
   created: <YYYY-MM-DD>
   feedback: <feedback or omit the line if none>
   replay: oneshot
   ---

   ## Prompts

   <!-- prompt -->
   <first user prompt verbatim>
   ```

   - Each prompt block is preceded by a literal `<!-- prompt -->` line — this is
     how bench splits prompts, so include it before every prompt.
   - `replay: oneshot` is the default (all prompts collapsed into one). Use
     `replay: sequential` only if the user asks to preserve turn-by-turn replay.
   - Optionally add `expects: diff | answer | conversation` when the deliverable
     is clear: `answer` if the task is a question (file edits would be off-task),
     `conversation` if the feedback is about behavior across turns (e.g. "should
     have told me I was wrong early" — this forces sequential replay), `diff`
     for pure code changes. Omit it otherwise; the judge infers.

4. **Confirm** to the user: print the path written, the title, the anchor
   (repo@commit, or "scratch"), the number of prompts, and any detected gates.
   Tell them they can run it with `bench run`.

## Notes

- Never invent or paraphrase prompts — capture them exactly.
- The eval is self-contained: prompts + optional repo/commit + feedback rubric.
  There is no reference solution; the judge scores against the feedback note and
  (when present) the deterministic gates.
- If you cannot run git at all (no shell access), treat it as a scratch eval.
