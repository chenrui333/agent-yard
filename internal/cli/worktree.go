package cli

import (
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
	if item.Branch == "" {
		return fmt.Errorf("task %q has no branch", taskID)
	}
	repo := config.RepoPath(a.configPath, cfg)
	if stat, err := os.Stat(repo); err != nil {
		return fmt.Errorf("repo path %s is not accessible: %w", repo, err)
	} else if !stat.IsDir() {
		return fmt.Errorf("repo path %s is not a directory", repo)
	}
	if err := gitx.EnsureExists(); err != nil {
		return err
	}
	git := gitx.New()
	ctx := cmd.Context()
	if err := git.Fetch(ctx, repo, cfg.DefaultRemote); err != nil {
		return err
	}
	worktreePath := a.taskWorktreePath(cfg, *item)
	if worktreePath == "" {
		return fmt.Errorf("task %q has no worktree and branch could not derive one", taskID)
	}
	if stat, err := os.Stat(worktreePath); err == nil {
		if !stat.IsDir() {
			return fmt.Errorf("worktree path %s exists and is not a directory", worktreePath)
		}
		dirty, err := git.IsDirty(ctx, worktreePath)
		if err != nil {
			return fmt.Errorf("worktree path %s exists but is not a usable git worktree: %w", worktreePath, err)
		}
		if dirty {
			return fmt.Errorf("worktree path %s already exists and is dirty", worktreePath)
		}
		a.printf("worktree already exists: %s\n", worktreePath)
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
			return fmt.Errorf("create worktree parent: %w", err)
		}
		if err := git.AddWorktree(ctx, repo, item.Branch, worktreePath, cfg.DefaultRemote, cfg.BaseBranch); err != nil {
			return err
		}
		a.printf("created worktree: %s\n", worktreePath)
	} else {
		return fmt.Errorf("stat worktree path %s: %w", worktreePath, err)
	}
	item.Worktree = worktreePath
	item.Status = task.StatusWorktreeCreated
	if err := store.Save(ledger); err != nil {
		return err
	}
	return nil
}
