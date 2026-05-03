package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/execx"
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
	cmd := &cobra.Command{
		Use:   "review-pr <pr-number>",
		Short: "Launch a no-push PR review lane",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PR number %q", args[0])
			}
			return a.runReviewPR(cmd, prNumber, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the review command without launching")
	cmd.Flags().BoolVar(&opts.force, "force", false, "reuse an existing review window")
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

func (a *App) runReviewPR(cmd *cobra.Command, prNumber int, opts *launchOptions) error {
	cfg, err := a.loadConfig()
	if err != nil {
		return err
	}
	repo := config.RepoPath(a.configPath, cfg)
	if stat, err := os.Stat(repo); err != nil {
		return fmt.Errorf("repo path %s is not accessible: %w", repo, err)
	} else if !stat.IsDir() {
		return fmt.Errorf("repo path %s is not a directory", repo)
	}
	if !opts.force {
		dirty, err := gitx.New().IsDirty(cmd.Context(), repo)
		if err != nil {
			return err
		}
		if dirty {
			return fmt.Errorf("repo %s is dirty", repo)
		}
	}
	item := prReviewTask(prNumber)
	item.Worktree = repo
	window := agent.ReviewWindowName("pr-review", strconv.Itoa(prNumber))
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
	launchCommand := agent.BuildLaunchCommand(repo, promptPath, cfg.Agents.PRReview)
	if opts.dryRun {
		a.printf("window: %s\ncommand: %s\n", window, launchCommand)
		return nil
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
