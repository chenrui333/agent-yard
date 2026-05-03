package cli

import (
	"fmt"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

func (a *App) newPRCmd() *cobra.Command {
	var dryRun bool
	var title string
	cmd := &cobra.Command{
		Use:   "pr <task-id>",
		Short: "Open a focused GitHub pull request for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runPR(cmd, args[0], title, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print PR title/body without creating a PR")
	cmd.Flags().StringVar(&title, "title", "", "PR title override")
	return cmd
}

func (a *App) runPR(cmd *cobra.Command, taskID, title string, dryRun bool) error {
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
	if title == "" {
		title = defaultPRTitle(*item)
	}
	body := prBody(cfg, *item)
	if dryRun {
		a.printf("title: %s\nbase: %s\nhead: %s\nbody:\n%s\n", title, cfg.BaseBranch, item.Branch, body)
		return nil
	}
	if err := ghx.EnsureExists(); err != nil {
		return err
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
