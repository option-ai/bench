package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// repoURL normalizes a snapshot repo field into a clonable URL. Accepts full
// URLs, scp-style git@ remotes, and bare "github.com/owner/name" shorthand.
func repoURL(repo string) string {
	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") ||
		strings.HasPrefix(repo, "git@") || strings.HasPrefix(repo, "ssh://") {
		return repo
	}
	return "https://" + strings.TrimSuffix(repo, "/") + ".git"
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return out, nil
}

func gitClone(ctx context.Context, repo, dest string) error {
	_, err := git(ctx, "", "clone", "--quiet", repoURL(repo), dest)
	return err
}

func gitFetch(ctx context.Context, dir string) error {
	_, err := git(ctx, dir, "fetch", "--all", "--quiet", "--tags")
	return err
}

// gitWorktreeAdd adds a detached worktree at commit, fetching first if the
// commit isn't present in the cache yet.
func gitWorktreeAdd(ctx context.Context, repoDir, wt, commit string) error {
	if _, err := git(ctx, repoDir, "cat-file", "-e", commit+"^{commit}"); err != nil {
		_ = gitFetch(ctx, repoDir)
	}
	_ = os.RemoveAll(wt) // a stale worktree from a prior run would block add
	_, _ = git(ctx, repoDir, "worktree", "prune")
	_, err := git(ctx, repoDir, "worktree", "add", "--detach", "--force", wt, commit)
	return err
}

// gitCaptureDiff stages everything (so new files show) and returns the diff
// against the checked-out commit.
func gitCaptureDiff(ctx context.Context, wt string) (string, error) {
	if _, err := git(ctx, wt, "add", "-A"); err != nil {
		return "", err
	}
	out, err := git(ctx, wt, "diff", "--cached")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// scratchWorkspace creates a fresh, empty git repo for a scratch (repo-less)
// eval, with an empty initial commit so a later `diff --cached` shows every
// file the agent creates. Identity is set inline so the commit needs no global
// git config.
func scratchWorkspace(ctx context.Context, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := git(ctx, dir, "init", "--quiet"); err != nil {
		return err
	}
	_, err := git(ctx, dir,
		"-c", "user.email=bench@local", "-c", "user.name=bench",
		"commit", "--allow-empty", "--quiet", "-m", "bench scratch baseline")
	return err
}
