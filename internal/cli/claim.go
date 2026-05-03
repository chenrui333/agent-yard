package cli

import (
	"fmt"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

func (a *App) newClaimCmd() *cobra.Command {
	var assigned string
	var comment bool
	cmd := &cobra.Command{
		Use:   "claim <task-id>",
		Short: "Mark a task as claimed and optionally comment on the issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runClaim(cmd, args[0], assigned, comment)
		},
	}
	cmd.Flags().StringVar(&assigned, "agent", "", "assigned implementation lane or agent name")
	cmd.Flags().BoolVar(&comment, "comment", false, "post the claim comment to GitHub")
	return cmd
}

func (a *App) runClaim(cmd *cobra.Command, taskID, assigned string, comment bool) error {
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	issue := taskIssue(cfg, *item)
	body := fmt.Sprintf("Claiming task %s", taskID)
	if assigned != "" {
		body += fmt.Sprintf(" for %s", assigned)
	}
	body += "."
	if err := store.Update(taskID, func(current *task.Task) error {
		current.Status = task.StatusClaimed
		if assigned != "" {
			current.AssignedAgent = assigned
		}
		return nil
	}); err != nil {
		return err
	}
	if !comment {
		a.printf("not posting GitHub comment without --comment\nintended comment for issue #%d:\n%s\n", issue, body)
		return nil
	}
	if err := ghx.EnsureExists(); err != nil {
		return err
	}
	return ghx.New().IssueComment(cmd.Context(), config.RepoPath(a.configPath, cfg), config.GitHubRepoArg(cfg), issue, body)
}
