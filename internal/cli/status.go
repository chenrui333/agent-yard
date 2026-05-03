package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/chenrui333/agent-yard/internal/gitx"
	statusx "github.com/chenrui333/agent-yard/internal/status"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

func (a *App) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show task, worktree, tmux, and PR status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, ledger, _, err := a.loadState()
			if err != nil {
				return err
			}
			rows := a.collectStatusRows(cmd, cfg, ledger)
			statusx.RenderSummary(a.out, rows)
			return nil
		},
	}
}

func (a *App) newBoardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "board",
		Short: "Show a compact status board grouped by task state",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, ledger, _, err := a.loadState()
			if err != nil {
				return err
			}
			rows := a.collectStatusRows(cmd, cfg, ledger)
			statusx.RenderBoard(a.out, rows)
			return nil
		},
	}
}

func (a *App) collectStatusRows(cmd *cobra.Command, cfg config.Config, ledger task.Ledger) []statusx.Row {
	ctx := cmd.Context()
	git := gitx.New()
	tmuxClient := tmux.New()
	gh := ghx.New()
	repo := config.RepoPath(a.configPath, cfg)
	repoArg := config.GitHubRepoArg(cfg)
	var rows []statusx.Row
	for _, item := range ledger.Tasks {
		worktreePath := a.taskWorktreePath(cfg, item)
		row := statusx.Row{
			TaskID:       item.ID,
			LedgerStatus: item.Status,
			Branch:       item.Branch,
			Worktree:     worktreePath,
			Dirty:        "unknown",
			Tmux:         "unknown",
			PR:           prLabel(item),
			CIReview:     "unknown",
		}
		if stat, err := os.Stat(worktreePath); err == nil && stat.IsDir() {
			row.WorktreeOK = true
			if dirty, err := git.IsDirty(ctx, worktreePath); err == nil {
				if dirty {
					row.Dirty = "dirty"
				} else {
					row.Dirty = "clean"
				}
			}
		} else {
			row.Dirty = "n/a"
		}
		window := agent.TaskWindowName(item)
		if exists, err := tmuxClient.WindowExists(ctx, cfg.Session, window); err == nil {
			if exists {
				row.Tmux = window
			} else {
				row.Tmux = "missing"
			}
		}
		if item.PRNumber > 0 {
			if pr, err := gh.PRView(ctx, repo, repoArg, item.PRNumber); err == nil {
				row.CIReview = fmt.Sprintf("%s/%s", emptyAs(pr.MergeStateStatus, "unknown"), emptyAs(pr.ReviewDecision, "review-unknown"))
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func prLabel(item task.Task) string {
	if item.PRNumber > 0 && item.PRURL != "" {
		return fmt.Sprintf("#%d %s", item.PRNumber, item.PRURL)
	}
	if item.PRNumber > 0 {
		return "#" + strconv.Itoa(item.PRNumber)
	}
	return "-"
}

func emptyAs(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
