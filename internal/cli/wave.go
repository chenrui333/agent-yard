package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/prompt"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/chenrui333/agent-yard/internal/wave"
	"github.com/spf13/cobra"
)

func (a *App) newWaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wave",
		Short: "Plan, prepare, and launch implementation waves",
	}
	cmd.AddCommand(
		a.newWavePlanCmd(),
		a.newWavePrepareCmd(),
		a.newWaveLaunchCmd(),
	)
	return cmd
}

func (a *App) newWavePlanCmd() *cobra.Command {
	limit := 10
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan a wave using distinct service families when possible",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runWavePlan(limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum tasks to select")
	return cmd
}

func (a *App) newWavePrepareCmd() *cobra.Command {
	limit := 10
	comment := false
	dryRun := false
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Claim lanes and create worktrees for a planned wave",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runWavePrepare(cmd, limit, comment, dryRun)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum tasks to prepare")
	cmd.Flags().BoolVar(&comment, "comment", false, "post claim comments to the configured GitHub issue")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the selected tasks without mutating tasks or worktrees")
	return cmd
}

func (a *App) newWaveLaunchCmd() *cobra.Command {
	opts := &launchOptions{}
	limit := 10
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch a prepared implementation wave",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runWaveLaunch(cmd, opts, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum tasks to launch")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print tmux commands without launching")
	cmd.Flags().BoolVar(&opts.force, "force", false, "launch even if dirty worktrees or tmux windows exist")
	return cmd
}

func (a *App) runWavePlan(limit int) error {
	if limit < 1 {
		return fmt.Errorf("--limit must be greater than zero")
	}
	_, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	selections := wave.SelectTasks(ledger, wave.Options{
		Limit:                       limit,
		EligibleStatuses:            wave.Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	return a.renderWaveSelections(selections)
}

func (a *App) runWavePrepare(cmd *cobra.Command, limit int, comment, dryRun bool) error {
	if limit < 1 {
		return fmt.Errorf("--limit must be greater than zero")
	}
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	if comment {
		if err := ghx.EnsureExists(); err != nil {
			return err
		}
	}
	if dryRun {
		selections := wave.SelectTasks(ledger, wave.Options{
			Limit:                       limit,
			EligibleStatuses:            wave.Eligible(task.StatusReady),
			PreferDistinctServiceFamily: true,
		})
		return a.renderWaveSelections(selections)
	}

	selections := wave.SelectTasks(ledger, wave.Options{
		Limit:                       limit,
		EligibleStatuses:            wave.Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	originals := map[string]task.Task{}
	for _, selected := range selections {
		originals[selected.Task.ID] = selected.Task
	}
	if len(selections) == 0 {
		a.printf("selected 0 task(s)\n")
		return nil
	}

	prepared := 0
	var failures []string
	fetched := false
	for _, selected := range selections {
		worktreePath, _, err := a.ensureTaskWorktree(cmd.Context(), cfg, selected.Task, !fetched)
		if err != nil {
			a.printf("skip %s: %v\n", selected.Task.ID, err)
			failures = append(failures, selected.Task.ID)
			continue
		}
		fetched = true
		if err := store.Update(selected.Task.ID, func(item *task.Task) error {
			if item.Status != task.StatusReady {
				return fmt.Errorf("task %q status changed to %s", item.ID, item.Status)
			}
			item.AssignedAgent = selected.Lane
			item.Worktree = worktreePath
			item.Status = task.StatusWorktreeCreated
			return nil
		}); err != nil {
			return err
		}
		if comment {
			body := fmt.Sprintf("Claiming task %s for %s.", selected.Task.ID, selected.Lane)
			if err := ghx.New().IssueComment(cmd.Context(), config.RepoPath(a.configPath, cfg), config.GitHubRepoArg(cfg), taskIssue(cfg, selected.Task), body); err != nil {
				failures = append(failures, selected.Task.ID)
				if updateErr := a.rollbackPreparedTask(store, selected.Task.ID, originals[selected.Task.ID]); updateErr != nil {
					return updateErr
				}
				break
			}
		}
		prepared++
	}
	a.printf("prepared %d task(s)\n", prepared)
	if len(failures) > 0 {
		return fmt.Errorf("failed to prepare %d task(s): %s", len(failures), strings.Join(failures, ", "))
	}
	return nil
}

func (a *App) rollbackPreparedTask(store task.Store, taskID string, original task.Task) error {
	return store.Update(taskID, func(item *task.Task) error {
		item.Status = original.Status
		item.AssignedAgent = original.AssignedAgent
		item.Worktree = original.Worktree
		return nil
	})
}

func (a *App) runWaveLaunch(cmd *cobra.Command, opts *launchOptions, limit int) error {
	if limit < 1 {
		return fmt.Errorf("--limit must be greater than zero")
	}
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	launchable, err := a.launchableWaveLedger(cmd.Context(), cfg, ledger, opts)
	if err != nil {
		return err
	}
	selections := wave.SelectTasks(launchable, wave.Options{
		Limit:                       limit,
		EligibleStatuses:            wave.Eligible(task.StatusClaimed, task.StatusWorktreeCreated),
		PreferDistinctServiceFamily: true,
	})
	launched := 0
	for _, selected := range selections {
		item := selected.Task
		item.AssignedAgent = selected.Lane
		if err := a.launchTask(cmd, cfg, store, item, prompt.KindImplement, agent.TaskWindowName(item), cfg.Agents.Implementation, task.StatusRunning, opts); err != nil {
			a.printf("skip %s: %v\n", item.ID, err)
			continue
		}
		launched++
	}
	a.printf("selected %d task(s)\n", launched)
	return nil
}

func (a *App) launchableWaveLedger(ctx context.Context, cfg config.Config, ledger task.Ledger, opts *launchOptions) (task.Ledger, error) {
	launchable := task.EmptyLedger()
	git := gitx.New()
	tmuxClient := tmux.New()
	checkTmux := !opts.force && !opts.dryRun
	for _, item := range ledger.Tasks {
		if item.Status != task.StatusClaimed && item.Status != task.StatusWorktreeCreated {
			continue
		}
		worktreePath := a.taskWorktreePath(cfg, item)
		if worktreePath == "" {
			continue
		}
		worktreePath, err := filepath.Abs(worktreePath)
		if err != nil {
			return task.Ledger{}, err
		}
		stat, err := os.Stat(worktreePath)
		if err != nil || !stat.IsDir() {
			continue
		}
		if !opts.force {
			dirty, err := git.IsDirty(ctx, worktreePath)
			if err != nil || dirty {
				continue
			}
		}
		if checkTmux {
			exists, err := tmuxClient.WindowExists(ctx, cfg.Session, agent.TaskWindowName(item))
			if err != nil {
				return task.Ledger{}, err
			}
			if exists {
				continue
			}
		}
		launchable.Tasks = append(launchable.Tasks, item)
	}
	return launchable, nil
}

func (a *App) renderWaveSelections(selections []wave.Selection) error {
	tw := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TASK\tSTATUS\tFAMILY\tLANE\tBRANCH\tREASON\tWARNINGS"); err != nil {
		return err
	}
	for _, selected := range selections {
		warnings := "-"
		if len(selected.Warnings) > 0 {
			warnings = strings.Join(selected.Warnings, "; ")
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			selected.Task.ID,
			selected.Task.Status,
			emptyAs(selected.Task.ServiceFamily, "-"),
			selected.Lane,
			emptyAs(selected.Task.Branch, "-"),
			selected.Reason,
			warnings,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
