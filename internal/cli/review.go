package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/execx"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/prompt"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

func (a *App) newReviewLocalCmd() *cobra.Command {
	opts := &launchOptions{}
	cmd := &cobra.Command{
		Use:   "review-local <task-id>",
		Short: "Launch a read-only local review lane for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runReviewLocal(cmd, args[0], opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the review command without launching")
	cmd.Flags().BoolVar(&opts.force, "force", false, "reuse an existing review window")
	return cmd
}

func (a *App) newReviewPRCmd() *cobra.Command {
	opts := &launchOptions{}
	lane := "pr-review-a"
	cmd := &cobra.Command{
		Use:   "review-pr <pr-number>",
		Short: "Launch a no-push PR review lane",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PR number %q", args[0])
			}
			return a.runReviewPR(cmd, prNumber, lane, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the review command without launching")
	cmd.Flags().BoolVar(&opts.force, "force", false, "reuse an existing review window")
	cmd.Flags().StringVar(&lane, "lane", lane, "review lane name")
	return cmd
}

func (a *App) runReviewLocal(cmd *cobra.Command, taskID string, opts *launchOptions) error {
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	window := agent.ReviewWindowName("local-review", taskID)
	return a.launchTask(cmd, cfg, store, *item, prompt.KindLocalReview, window, cfg.Agents.LocalReview, task.StatusReviewPending, opts)
}

func (a *App) runReviewPR(cmd *cobra.Command, prNumber int, lane string, opts *launchOptions) error {
	cfg, err := a.loadConfig()
	if err != nil {
		return err
	}
	lane = agent.SanitizeWindowName(lane)
	if lane == "" {
		return fmt.Errorf("lane is required")
	}
	reviewWorktree := a.prReviewWorktreePath(prNumber, lane)
	item := prReviewTask(prNumber)
	item.ID = fmt.Sprintf("pr-%d-%s", prNumber, lane)
	item.Worktree = reviewWorktree
	window := agent.ReviewWindowName("pr-review", fmt.Sprintf("%d-%s", prNumber, lane))
	promptPath := a.promptFile(prompt.KindPRReview, item.ID)
	renderer := prompt.Renderer{Dir: a.promptDir()}
	data := prompt.Data{Config: cfg, Task: item, PRNumber: prNumber}
	if opts.dryRun {
		if _, err := renderer.Render(prompt.KindPRReview, data); err != nil {
			return err
		}
	} else if err := renderer.RenderToFile(prompt.KindPRReview, data, promptPath); err != nil {
		return err
	}
	launchCommand := agent.BuildLaunchCommand(reviewWorktree, promptPath, cfg.Agents.PRReview)
	if opts.dryRun {
		a.printf("worktree: %s\ncheckout: gh pr checkout %d --detach\nwindow: %s\ncommand: %s\n", reviewWorktree, prNumber, window, launchCommand)
		return nil
	}
	if _, err := a.ensurePRReviewWorktree(cmd.Context(), cfg, prNumber, lane, opts.force); err != nil {
		return err
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	if _, err := execx.LookPath(cfg.Agents.PRReview.Command); err != nil {
		return err
	}
	tmuxClient := tmux.New()
	if err := tmuxClient.EnsureSession(cmd.Context(), cfg.Session); err != nil {
		return err
	}
	exists, err := tmuxClient.WindowExists(cmd.Context(), cfg.Session, window)
	if err != nil {
		return err
	}
	if exists && !opts.force {
		return fmt.Errorf("tmux window %s already exists", window)
	}
	if !exists {
		if err := tmuxClient.NewWindow(cmd.Context(), cfg.Session, window); err != nil {
			return err
		}
	}
	return tmuxClient.SendKeys(cmd.Context(), tmux.Target(cfg.Session, window), launchCommand)
}

func (a *App) prReviewWorktreePath(prNumber int, lane string) string {
	return a.yardPath("reviews", fmt.Sprintf("pr-%d-%s", prNumber, agent.SanitizeWindowName(lane)))
}

func (a *App) ensurePRReviewWorktree(ctx context.Context, cfg config.Config, prNumber int, lane string, force bool) (string, error) {
	repo := config.RepoPath(a.configPath, cfg)
	if stat, err := os.Stat(repo); err != nil {
		return "", fmt.Errorf("repo path %s is not accessible: %w", repo, err)
	} else if !stat.IsDir() {
		return "", fmt.Errorf("repo path %s is not a directory", repo)
	}
	if err := gitx.EnsureExists(); err != nil {
		return "", err
	}
	if err := ghx.EnsureExists(); err != nil {
		return "", err
	}
	git := gitx.New()
	reviewWorktree := a.prReviewWorktreePath(prNumber, lane)
	if stat, err := os.Stat(reviewWorktree); err == nil {
		if !stat.IsDir() {
			return "", fmt.Errorf("review worktree path %s exists and is not a directory", reviewWorktree)
		}
		if !force {
			dirty, err := git.IsDirty(ctx, reviewWorktree)
			if err != nil {
				return "", fmt.Errorf("review worktree %s is not a usable git worktree: %w", reviewWorktree, err)
			}
			if dirty {
				return "", fmt.Errorf("review worktree %s is dirty", reviewWorktree)
			}
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(reviewWorktree), 0o755); err != nil {
			return "", fmt.Errorf("create review worktree parent: %w", err)
		}
		if err := git.Fetch(ctx, repo, cfg.DefaultRemote); err != nil {
			return "", err
		}
		if err := git.AddDetachedWorktree(ctx, repo, reviewWorktree, cfg.DefaultRemote, cfg.BaseBranch); err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("stat review worktree path %s: %w", reviewWorktree, err)
	}
	if err := ghx.New().PRCheckout(ctx, reviewWorktree, config.GitHubRepoArg(cfg), prNumber, true); err != nil {
		return "", err
	}
	return reviewWorktree, nil
}
