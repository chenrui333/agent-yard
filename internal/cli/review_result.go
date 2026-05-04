package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	reviewResultClear    = "clear"
	reviewResultFindings = "findings"
)

type reviewResultOptions struct {
	lane       string
	status     string
	priorities []string
	summary    string
}

type reviewResult struct {
	PRNumber   int      `yaml:"pr_number"`
	TaskID     string   `yaml:"task_id"`
	Lane       string   `yaml:"lane"`
	Head       string   `yaml:"head,omitempty"`
	Status     string   `yaml:"status"`
	Priorities []string `yaml:"priorities,omitempty"`
	Summary    string   `yaml:"summary,omitempty"`
	RecordedAt string   `yaml:"recorded_at"`
}

func (a *App) newReviewResultCmd() *cobra.Command {
	opts := &reviewResultOptions{lane: "pr-review-a", status: reviewResultClear}
	cmd := &cobra.Command{
		Use:   "review-result <task-id>",
		Short: "Record a structured PR review result for readiness checks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runReviewResult(cmd, args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.lane, "lane", opts.lane, "paired PR review lane")
	cmd.Flags().StringVar(&opts.status, "status", opts.status, "review result status: clear or findings")
	cmd.Flags().StringArrayVar(&opts.priorities, "priority", nil, "finding priority such as P1, P2, or P3; repeatable")
	cmd.Flags().StringVar(&opts.summary, "summary", "", "short review result summary")
	return cmd
}

func (a *App) runReviewResult(cmd *cobra.Command, taskID string, opts *reviewResultOptions) error {
	cfg, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	prNumber := item.PRNumber
	if prNumber == 0 {
		prNumber = prNumberFromTaskURL(*item)
	}
	if prNumber == 0 {
		return fmt.Errorf("task %q has no PR number", taskID)
	}
	status, err := normalizeReviewResultStatus(opts.status)
	if err != nil {
		return err
	}
	priorities, err := normalizeReviewPriorities(opts.priorities)
	if err != nil {
		return err
	}
	if status == reviewResultClear {
		if blocking := blockingReviewPriorities(priorities); len(blocking) > 0 {
			return fmt.Errorf("review result status %q cannot include blocking priorities %s; use --status %s or omit --priority", reviewResultClear, strings.Join(blocking, ","), reviewResultFindings)
		}
	}
	head, err := a.reviewResultHead(cmd.Context(), cfg, *item, prNumber, opts.lane)
	if err != nil {
		return err
	}
	result := reviewResult{
		PRNumber:   prNumber,
		TaskID:     taskID,
		Lane:       reviewLaneWindow(prNumber, opts.lane),
		Head:       head,
		Status:     status,
		Priorities: priorities,
		Summary:    strings.TrimSpace(opts.summary),
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
	}
	path := a.reviewResultPath(prNumber, opts.lane)
	if err := a.saveReviewResult(path, result); err != nil {
		return err
	}
	a.printf("recorded review result: %s\n", path)
	return nil
}

func (a *App) reviewResultHead(ctx context.Context, cfg config.Config, item task.Task, prNumber int, lane string) (string, error) {
	git := gitx.New()
	reviewWorktree, err := filepath.Abs(a.prReviewWorktreePath(prNumber, prReviewLaneName(prNumber, lane)))
	if err != nil {
		return "", err
	}
	if exists, err := validateReviewWorktreeRoot(ctx, git, reviewWorktree); err != nil {
		return "", err
	} else if exists {
		dirty, err := git.IsDirty(ctx, reviewWorktree)
		if err != nil {
			return "", fmt.Errorf("check review worktree dirty state %s: %w", reviewWorktree, err)
		}
		if dirty {
			return "", fmt.Errorf("review worktree %s is dirty", reviewWorktree)
		}
		head, err := git.RevParse(ctx, reviewWorktree, "HEAD")
		if err != nil {
			return "", fmt.Errorf("resolve review worktree HEAD in %s: %w", reviewWorktree, err)
		}
		return head, nil
	}

	worktreePath := a.taskWorktreePath(cfg, item)
	if worktreePath == "" {
		return "", fmt.Errorf("task %q has no worktree", item.ID)
	}
	worktreePath, err = filepath.Abs(worktreePath)
	if err != nil {
		return "", err
	}
	head, err := git.RevParse(ctx, worktreePath, "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve task HEAD in %s: %w", worktreePath, err)
	}
	return head, nil
}

func prReviewLaneName(prNumber int, lane string) string {
	lane = agent.SanitizeWindowName(lane)
	prefix := fmt.Sprintf("pr-review-%d-", prNumber)
	if strings.HasPrefix(lane, prefix) {
		lane = strings.TrimPrefix(lane, prefix)
	}
	return lane
}

func normalizeReviewResultStatus(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case reviewResultClear:
		return reviewResultClear, nil
	case reviewResultFindings:
		return reviewResultFindings, nil
	default:
		return "", fmt.Errorf("invalid review result status %q", value)
	}
}

func normalizeReviewPriorities(values []string) ([]string, error) {
	priorities := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = normalizeReviewPriorityToken(value)
		if value == "" {
			continue
		}
		switch value {
		case "P1", "P2", "P3":
		default:
			return nil, fmt.Errorf("invalid review priority %q; expected P1, P2, or P3", value)
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		priorities = append(priorities, value)
	}
	return priorities, nil
}

func normalizeReviewPriorityToken(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.Trim(value, "[](){}")
	return strings.ToUpper(strings.TrimSpace(value))
}

func (a *App) reviewResultPath(prNumber int, lane string) string {
	return a.yardPath("review-results", reviewLaneWindow(prNumber, lane)+".yaml")
}

func (a *App) saveReviewResult(path string, result reviewResult) error {
	data, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal review result: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create review result dir: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func (a *App) loadReviewResult(prNumber int, lane string) (reviewResult, bool, error) {
	path := a.reviewResultPath(prNumber, lane)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return reviewResult{}, false, nil
	}
	if err != nil {
		return reviewResult{}, false, fmt.Errorf("read review result %s: %w", path, err)
	}
	var result reviewResult
	if err := yaml.Unmarshal(data, &result); err != nil {
		return reviewResult{}, false, fmt.Errorf("parse review result %s: %w", path, err)
	}
	return result, true, nil
}

func reviewResultDetail(result reviewResult) string {
	parts := []string{result.Lane, result.Status}
	if result.Head != "" {
		parts = append(parts, result.Head)
	}
	if len(result.Priorities) > 0 {
		parts = append(parts, strings.Join(result.Priorities, ","))
	}
	if result.Summary != "" {
		parts = append(parts, result.Summary)
	}
	return strings.Join(parts, " ")
}
