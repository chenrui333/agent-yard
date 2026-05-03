package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

func (a *App) newWorktreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "worktree <task-id>",
		Short: "Create or verify a git worktree for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runWorktree(cmd, args[0])
		},
	}
}

func (a *App) runWorktree(cmd *cobra.Command, taskID string) error {
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	worktreePath, _, err := a.ensureTaskWorktree(cmd.Context(), cfg, *item, true)
	if err != nil {
		return err
	}
	return store.Update(taskID, func(current *task.Task) error {
		current.Worktree = worktreePath
		current.Status = task.StatusWorktreeCreated
		return nil
	})
}

func (a *App) ensureTaskWorktree(ctx context.Context, cfg config.Config, item task.Task, fetch bool) (string, bool, error) {
	if item.Branch == "" {
		return "", false, fmt.Errorf("task %q has no branch", item.ID)
	}
	repo := config.RepoPath(a.configPath, cfg)
	if stat, err := os.Stat(repo); err != nil {
		return "", false, fmt.Errorf("repo path %s is not accessible: %w", repo, err)
	} else if !stat.IsDir() {
		return "", false, fmt.Errorf("repo path %s is not a directory", repo)
	}
	if err := gitx.EnsureExists(); err != nil {
		return "", false, err
	}
	git := gitx.New()
	worktreePath := a.taskWorktreePath(cfg, item)
	if worktreePath == "" {
		return "", false, fmt.Errorf("task %q has no worktree and branch could not derive one", item.ID)
	}
	worktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return "", false, err
	}
	if stat, err := os.Stat(worktreePath); err == nil {
		if !stat.IsDir() {
			return "", false, fmt.Errorf("worktree path %s exists and is not a directory", worktreePath)
		}
		dirty, err := git.IsDirty(ctx, worktreePath)
		if err != nil {
			return "", false, fmt.Errorf("worktree path %s exists but is not a usable git worktree: %w", worktreePath, err)
		}
		if dirty {
			return "", false, fmt.Errorf("worktree path %s already exists and is dirty", worktreePath)
		}
		a.printf("worktree already exists: %s\n", worktreePath)
		return worktreePath, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("stat worktree path %s: %w", worktreePath, err)
	}
	if fetch {
		if err := git.Fetch(ctx, repo, cfg.DefaultRemote); err != nil {
			return "", false, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return "", false, fmt.Errorf("create worktree parent: %w", err)
	}
	if err := git.AddWorktree(ctx, repo, item.Branch, worktreePath, cfg.DefaultRemote, cfg.BaseBranch); err != nil {
		return "", false, err
	}
	a.printf("created worktree: %s\n", worktreePath)
	return worktreePath, true, nil
}
