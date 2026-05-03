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
			rows := a.collectStatusRows(cmd, cfg, ledger, statusOptions{includeRemote: true})
			return statusx.RenderSummary(a.out, rows)
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
			rows := a.collectStatusRows(cmd, cfg, ledger, statusOptions{})
			return statusx.RenderBoard(a.out, rows)
		},
	}
}

type statusOptions struct {
	includeRemote bool
}

func (a *App) collectStatusRows(cmd *cobra.Command, cfg config.Config, ledger task.Ledger, opts statusOptions) []statusx.Row {
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
			AheadBehind:  "unknown",
			ChangedFiles: "unknown",
			Remote:       "unknown",
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
			baseRef := cfg.DefaultRemote + "/" + cfg.BaseBranch
			if aheadBehind, err := git.AheadBehind(ctx, worktreePath, baseRef); err == nil {
				row.AheadBehind = fmt.Sprintf("+%d/-%d", aheadBehind.Ahead, aheadBehind.Behind)
			}
			if files, err := git.ChangedFilesSince(ctx, worktreePath, baseRef); err == nil {
				row.ChangedFiles = strconv.Itoa(len(files))
			}
			if opts.includeRemote {
				if item.Branch == "" {
					row.Remote = "n/a"
				} else if exists, err := git.RemoteTrackingBranchExists(ctx, worktreePath, cfg.DefaultRemote, item.Branch); err == nil {
					if exists {
						row.Remote = "pushed"
					} else {
						row.Remote = "local"
					}
				}
			} else {
				row.Remote = "n/a"
			}
		} else {
			row.Dirty = "n/a"
			row.AheadBehind = "n/a"
			row.ChangedFiles = "n/a"
			row.Remote = "n/a"
		}
		window := agent.TaskWindowName(item)
		if exists, err := tmuxClient.WindowExists(ctx, cfg.Session, window); err == nil {
			if exists {
				target := tmux.Target(cfg.Session, window)
				if panes, err := tmuxClient.ListPanes(ctx, target); err == nil {
					row.Tmux = paneStatus(panes)
				} else {
					row.Tmux = window
				}
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

func paneStatus(panes []tmux.Pane) string {
	if len(panes) == 0 {
		return "no panes"
	}
	pane := panes[0]
	if pane.Dead {
		return "dead exit=" + emptyAs(pane.DeadStatus, "unknown")
	}
	command := emptyAs(pane.CurrentCommand, "unknown")
	switch command {
	case "bash", "sh", "zsh", "fish":
		return "idle " + command
	default:
		return "running " + command
	}
}
