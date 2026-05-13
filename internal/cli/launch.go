package cli

import (
	"fmt"
	"os"
	"path/filepath"
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

type launchOptions struct {
	dryRun        bool
	force         bool
	reuseIdle     bool
	replaceWindow bool
}

func (a *App) newLaunchCmd() *cobra.Command {
	opts := &launchOptions{}
	cmd := &cobra.Command{
		Use:   "launch <task-id>",
		Short: "Launch an implementation agent for a task in tmux",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runLaunch(cmd, args[0], opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the tmux command without launching")
	cmd.Flags().BoolVar(&opts.force, "force", false, "launch even if the worktree is dirty")
	addWindowReuseFlags(cmd, opts)
	return cmd
}

func (a *App) newLaunchWaveCmd() *cobra.Command {
	opts := &launchOptions{}
	limit := 10
	cmd := &cobra.Command{
		Use:   "launch-wave",
		Short: "Compatibility alias for wave launch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runLaunchWave(cmd, opts, limit)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print tmux commands without launching")
	cmd.Flags().BoolVar(&opts.force, "force", false, "launch even if worktrees are dirty")
	addWindowReuseFlags(cmd, opts)
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum tasks to launch")
	return cmd
}

func (a *App) runLaunch(cmd *cobra.Command, taskID string, opts *launchOptions) error {
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	return a.launchTask(cmd, cfg, store, *item, prompt.KindImplement, agent.TaskWindowName(*item), cfg.Agents.Implementation, task.StatusRunning, opts)
}

func (a *App) runLaunchWave(cmd *cobra.Command, opts *launchOptions, limit int) error {
	a.printf("launch-wave is an alias for wave launch; prefer `yard wave launch`.\n")
	return a.runWaveLaunch(cmd, opts, limit)
}

func (a *App) launchTask(cmd *cobra.Command, cfg config.Config, store task.Store, item task.Task, kind, window string, command config.AgentCommand, nextStatus task.Status, opts *launchOptions) error {
	if opts == nil {
		opts = &launchOptions{}
	}
	worktreePath := a.taskWorktreePath(cfg, item)
	if worktreePath == "" {
		return fmt.Errorf("task %q has no worktree", item.ID)
	}
	worktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return err
	}
	if stat, err := os.Stat(worktreePath); err != nil {
		return fmt.Errorf("worktree %s is not accessible: %w", worktreePath, err)
	} else if !stat.IsDir() {
		return fmt.Errorf("worktree %s is not a directory", worktreePath)
	}
	if !opts.force {
		dirty, err := gitx.New().IsDirty(cmd.Context(), worktreePath)
		if err != nil {
			return err
		}
		if dirty {
			return fmt.Errorf("worktree %s is dirty", worktreePath)
		}
	}
	promptFile, err := a.promptFile(kind, item.ID)
	if err != nil {
		return err
	}
	promptPath, err := filepath.Abs(promptFile)
	if err != nil {
		return err
	}
	data := prompt.Data{
		Config:     cfg,
		Task:       item,
		Issue:      taskIssue(cfg, item),
		BaseBranch: cfg.BaseBranch,
		Remote:     cfg.DefaultRemote,
	}
	renderer := prompt.Renderer{Dir: a.promptDir()}
	if opts.dryRun {
		if _, err := renderer.Render(kind, data); err != nil {
			return err
		}
	} else {
		if err := renderer.RenderToFile(kind, data, promptPath); err != nil {
			return err
		}
	}
	launchCommand := agent.BuildLaunchCommand(worktreePath, promptPath, command)
	if opts.dryRun {
		a.printf("window: %s\ncommand: %s\n", window, launchCommand)
		return nil
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	if _, err := execx.LookPath(command.Command); err != nil {
		return err
	}
	tmuxClient := tmux.New()
	ctx := cmd.Context()
	if err := tmuxClient.EnsureSession(ctx, cfg.Session); err != nil {
		return err
	}
	windowPlan, err := planTmuxWindow(ctx, tmuxClient, cfg.Session, window, opts)
	if err != nil {
		return err
	}
	if err := executeTmuxWindowPlan(ctx, tmuxClient, windowPlan, launchCommand); err != nil {
		return err
	}
	return store.Update(item.ID, func(current *task.Task) error {
		current.Worktree = worktreePath
		if item.AssignedAgent != "" {
			current.AssignedAgent = item.AssignedAgent
		}
		current.Status = nextStatus
		return nil
	})
}

func prReviewTask(prNumber int) task.Task {
	id := "pr-" + strconv.Itoa(prNumber)
	return task.Task{ID: id, Status: task.StatusReviewPending}
}
