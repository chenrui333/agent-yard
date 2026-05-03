package cli

import (
	"fmt"
	"strconv"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/spf13/cobra"
)

func (a *App) newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync external control-plane state into the local ledger",
	}
	cmd.AddCommand(a.newSyncIssueCmd())
	return cmd
}

func (a *App) newSyncIssueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issue <issue-number>",
		Short: "Read a GitHub issue and show sync input for task extraction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue number %q", args[0])
			}
			cfg, err := a.loadConfig()
			if err != nil {
				return err
			}
			if err := ghx.EnsureExists(); err != nil {
				return err
			}
			issue, err := ghx.New().IssueView(cmd.Context(), config.RepoPath(a.configPath, cfg), config.GitHubRepoArg(cfg), issueNumber)
			if err != nil {
				return err
			}
			a.printf("issue #%d: %s\n%s\n\nTODO: checkbox-to-task reconciliation is deferred for MVP.\n", issueNumber, issue.Title, issue.URL)
			return nil
		},
	}
}
