package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

type prOptions struct {
	dryRun      bool
	title       string
	noPush      bool
	allowBehind bool
}

func (a *App) newPRCmd() *cobra.Command {
	opts := &prOptions{}
	cmd := &cobra.Command{
		Use:   "pr <task-id>",
		Short: "Open a focused GitHub pull request for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runPR(cmd, args[0], opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print PR preflights, push command, title, and body without creating a PR")
	cmd.Flags().StringVar(&opts.title, "title", "", "PR title override")
	cmd.Flags().BoolVar(&opts.noPush, "no-push", false, "do not push the task branch before creating the PR")
	cmd.Flags().BoolVar(&opts.allowBehind, "allow-behind", false, "allow creating a PR when the branch is behind the configured base ref")
	return cmd
}

func (a *App) runPR(cmd *cobra.Command, taskID string, opts *prOptions) error {
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	if item.Branch == "" {
		return fmt.Errorf("task %q has no branch", taskID)
	}
	title := opts.title
	if title == "" {
		title = defaultPRTitle(*item)
	}
	body := prBody(cfg, *item)
	git := gitx.New()
	if opts.dryRun {
		if _, err := a.prPreflight(cmd.Context(), cfg, *item, opts.allowBehind, false); err != nil {
			a.printf("preflight: would fail: %v\n", err)
		} else {
			a.printf("preflight: ok\n")
		}
		if opts.noPush {
			a.printf("push: skipped by --no-push\n")
		} else {
			a.printf("push: git push -u %s %s\n", cfg.DefaultRemote, item.Branch)
		}
		a.printf("title: %s\nbase: %s\nhead: %s\nbody:\n%s\n", title, cfg.BaseBranch, item.Branch, body)
		return nil
	}
	if err := ghx.EnsureExists(); err != nil {
		return err
	}
	if err := git.Fetch(cmd.Context(), config.RepoPath(a.configPath, cfg), cfg.DefaultRemote); err != nil {
		return err
	}
	worktreePath, err := a.prPreflight(cmd.Context(), cfg, *item, opts.allowBehind, true)
	if err != nil {
		return err
	}
	if !opts.noPush {
		if err := git.Push(cmd.Context(), worktreePath, cfg.DefaultRemote, item.Branch); err != nil {
			return err
		}
	}
	if pr, ok, err := ghx.New().PRForBranch(cmd.Context(), config.RepoPath(a.configPath, cfg), config.GitHubRepoArg(cfg), item.Branch); err != nil {
		return err
	} else if ok {
		if pr.BaseRefName != "" && pr.BaseRefName != cfg.BaseBranch {
			return fmt.Errorf("existing PR %s targets base %q, want %q", pr.URL, pr.BaseRefName, cfg.BaseBranch)
		}
		a.printf("existing PR: %s\n", pr.URL)
		return store.Update(taskID, func(current *task.Task) error {
			current.PRURL = pr.URL
			current.PRNumber = pr.Number
			current.Status = task.StatusPROpened
			return nil
		})
	}
	url, number, err := ghx.New().CreatePRWithBody(cmd.Context(), config.RepoPath(a.configPath, cfg), ghx.CreatePROptions{
		RepoArg: config.GitHubRepoArg(cfg),
		Title:   title,
		Base:    cfg.BaseBranch,
		Head:    item.Branch,
	}, body)
	if err != nil {
		return err
	}
	return store.Update(taskID, func(current *task.Task) error {
		current.PRURL = url
		current.PRNumber = number
		current.Status = task.StatusPROpened
		return nil
	})
}

func (a *App) prPreflight(ctx context.Context, cfg config.Config, item task.Task, allowBehind, strict bool) (string, error) {
	worktreePath := a.taskWorktreePath(cfg, item)
	if worktreePath == "" {
		return "", fmt.Errorf("task %q has no worktree", item.ID)
	}
	worktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return "", err
	}
	stat, err := os.Stat(worktreePath)
	if err != nil {
		return "", fmt.Errorf("worktree %s is not accessible: %w", worktreePath, err)
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("worktree %s is not a directory", worktreePath)
	}
	git := gitx.New()
	branch, err := git.BranchShowCurrent(ctx, worktreePath)
	if err != nil {
		return "", err
	}
	if branch != item.Branch {
		return "", fmt.Errorf("worktree branch is %q, want %q", branch, item.Branch)
	}
	dirty, err := git.IsDirty(ctx, worktreePath)
	if err != nil {
		return "", err
	}
	if dirty {
		return "", fmt.Errorf("worktree %s is dirty", worktreePath)
	}
	baseRef := cfg.DefaultRemote + "/" + cfg.BaseBranch
	if err := git.DiffCheckSince(ctx, worktreePath, baseRef); err != nil {
		return "", err
	}
	aheadBehind, err := git.AheadBehind(ctx, worktreePath, baseRef)
	if err != nil {
		if strict {
			return "", err
		}
		return worktreePath, nil
	}
	if aheadBehind.Ahead == 0 {
		return "", fmt.Errorf("branch %q has no commits ahead of %s", item.Branch, baseRef)
	}
	if aheadBehind.Behind > 0 && !allowBehind {
		return "", fmt.Errorf("branch %q is behind %s by %d commit(s)", item.Branch, baseRef, aheadBehind.Behind)
	}
	return worktreePath, nil
}

func defaultPRTitle(item task.Task) string {
	if item.Checkbox != "" {
		return item.Checkbox
	}
	return item.ID
}

func prBody(cfg config.Config, item task.Task) string {
	issue := taskIssue(cfg, item)
	body := "## Summary\n\n"
	if item.Checkbox != "" {
		body += "- " + item.Checkbox + "\n"
	} else {
		body += "- " + item.ID + "\n"
	}
	if issue > 0 {
		body += "\n## References\n\nRefs #" + fmt.Sprint(issue) + "\n"
	}
	return body
}
