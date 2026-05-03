package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

type readyOptions struct {
	reviewLane string
	write      bool
}

type readyCheck struct {
	Name   string
	Status string
	Detail string
}

func (a *App) newReadyCmd() *cobra.Command {
	opts := &readyOptions{}
	cmd := &cobra.Command{
		Use:   "ready <task-id>",
		Short: "Check whether a task PR is merge-ready",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runReady(cmd, args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.reviewLane, "review-lane", "", "paired PR review lane to inspect for P1/P2/P3 TODO findings")
	cmd.Flags().BoolVar(&opts.write, "write", false, "mark the task merge_ready when all required checks pass")
	return cmd
}

func (a *App) runReady(cmd *cobra.Command, taskID string, opts *readyOptions) error {
	cfg, ledger, store, err := a.loadState()
	if err != nil {
		return err
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	checks := a.collectReadyChecks(cmd, cfg, *item, opts)
	if err := a.renderReadyChecks(checks); err != nil {
		return err
	}
	if readyChecksFailed(checks) {
		return fmt.Errorf("task %q is not merge-ready", taskID)
	}
	if opts.write {
		return store.Update(taskID, func(current *task.Task) error {
			current.Status = task.StatusMergeReady
			return nil
		})
	}
	return nil
}

func (a *App) collectReadyChecks(cmd *cobra.Command, cfg config.Config, item task.Task, opts *readyOptions) []readyCheck {
	ctx := cmd.Context()
	git := gitx.New()
	checks := []readyCheck{}
	localWorktree := ""
	repoPath := config.RepoPath(a.configPath, cfg)
	baseRef := cfg.DefaultRemote + "/" + cfg.BaseBranch
	fetchedRemote := false
	add := func(name, status, detail string) {
		checks = append(checks, readyCheck{Name: name, Status: status, Detail: detail})
	}
	fetchRemote := func() error {
		if fetchedRemote {
			return nil
		}
		if err := git.Fetch(ctx, repoPath, cfg.DefaultRemote); err != nil {
			return err
		}
		fetchedRemote = true
		return nil
	}

	worktreePath := a.taskWorktreePath(cfg, item)
	if worktreePath == "" {
		add("worktree", "fail", "missing worktree path")
	} else if abs, err := filepath.Abs(worktreePath); err != nil {
		add("worktree", "fail", err.Error())
	} else if stat, err := os.Stat(abs); err != nil {
		add("worktree", "fail", err.Error())
	} else if !stat.IsDir() {
		add("worktree", "fail", "not a directory")
	} else {
		add("worktree", "pass", abs)
		if branch, err := git.BranchShowCurrent(ctx, abs); err != nil {
			add("branch", "fail", err.Error())
		} else if branch != item.Branch {
			add("branch", "fail", fmt.Sprintf("current branch %q, want %q", branch, item.Branch))
		} else {
			add("branch", "pass", branch)
		}
		if dirty, err := git.IsDirty(ctx, abs); err != nil {
			add("worktree clean", "fail", err.Error())
		} else if dirty {
			add("worktree clean", "fail", "dirty")
		} else {
			add("worktree clean", "pass", "clean")
		}
		if err := fetchRemote(); err != nil {
			add("diff check", "fail", err.Error())
		} else if err := git.DiffCheckSince(ctx, abs, baseRef); err != nil {
			add("diff check", "fail", err.Error())
		} else {
			add("diff check", "pass", "ok")
		}
		localWorktree = abs
	}

	if item.Branch == "" {
		add("remote branch", "fail", "task has no branch")
	} else if exists, err := git.RemoteBranchExists(ctx, repoPath, cfg.DefaultRemote, item.Branch); err != nil {
		add("remote branch", "fail", err.Error())
	} else if !exists {
		add("remote branch", "fail", "not pushed")
	} else if localWorktree != "" {
		remoteRef := cfg.DefaultRemote + "/" + item.Branch
		if err := fetchRemote(); err != nil {
			add("remote branch", "fail", err.Error())
		} else if contains, err := git.IsAncestor(ctx, localWorktree, "HEAD", remoteRef); err != nil {
			add("remote branch", "fail", err.Error())
		} else if !contains {
			add("remote branch", "fail", "local HEAD is not contained in "+remoteRef)
		} else {
			add("remote branch", "pass", "pushed")
		}
	} else {
		add("remote branch", "pass", "pushed")
	}

	pr, prOK := a.readyPullRequest(ctx, cfg, item, add)
	if prOK {
		addPRReadyChecks(pr, add)
		if opts.reviewLane != "" {
			a.addReviewLaneReadyCheck(ctx, cfg, pr.Number, opts.reviewLane, add)
		} else if opts.write {
			add("review lane", "fail", "--review-lane is required with --write")
		} else {
			add("review lane", "skip", "not requested")
		}
	}
	return checks
}

func (a *App) readyPullRequest(ctx context.Context, cfg config.Config, item task.Task, add func(string, string, string)) (ghx.PullRequest, bool) {
	if !githubConfigured(cfg) {
		add("pr", "fail", "GitHub owner/repo not configured")
		return ghx.PullRequest{}, false
	}
	if err := ghx.EnsureExists(); err != nil {
		add("pr", "fail", err.Error())
		return ghx.PullRequest{}, false
	}
	client := ghx.New()
	repoPath := config.RepoPath(a.configPath, cfg)
	repoArg := config.GitHubRepoArg(cfg)
	prNumber := item.PRNumber
	if prNumber == 0 {
		prNumber = ghx.PRNumberFromURL(item.PRURL)
	}
	if prNumber != 0 {
		pr, err := client.PRView(ctx, repoPath, repoArg, prNumber)
		if err != nil {
			add("pr", "fail", err.Error())
			return ghx.PullRequest{}, false
		}
		if item.Branch != "" && pr.HeadRefName != "" && pr.HeadRefName != item.Branch {
			add("pr", "fail", fmt.Sprintf("PR head branch %q, want %q", pr.HeadRefName, item.Branch))
			return ghx.PullRequest{}, false
		}
		if pr.BaseRefName != "" && pr.BaseRefName != cfg.BaseBranch {
			add("pr", "fail", fmt.Sprintf("PR base branch %q, want %q", pr.BaseRefName, cfg.BaseBranch))
			return ghx.PullRequest{}, false
		}
		add("pr", "pass", pr.URL)
		return pr, true
	}
	pr, ok, err := client.PRForBranch(ctx, repoPath, repoArg, item.Branch)
	if err != nil {
		add("pr", "fail", err.Error())
		return ghx.PullRequest{}, false
	}
	if !ok {
		add("pr", "fail", "no open PR for branch "+item.Branch)
		return ghx.PullRequest{}, false
	}
	if pr.BaseRefName != "" && pr.BaseRefName != cfg.BaseBranch {
		add("pr", "fail", fmt.Sprintf("PR base branch %q, want %q", pr.BaseRefName, cfg.BaseBranch))
		return ghx.PullRequest{}, false
	}
	add("pr", "pass", pr.URL)
	return pr, true
}

func addPRReadyChecks(pr ghx.PullRequest, add func(string, string, string)) {
	if pr.State != "" && pr.State != "OPEN" {
		add("pr state", "fail", pr.State)
	} else {
		add("pr state", "pass", emptyAs(pr.State, "OPEN"))
	}
	mergeState := emptyAs(pr.MergeStateStatus, "UNKNOWN")
	if mergeState == "CLEAN" || mergeState == "HAS_HOOKS" || mergeState == "UNSTABLE" {
		add("merge state", "pass", mergeState)
	} else {
		add("merge state", "fail", mergeState)
	}
	if pr.ReviewDecision == "CHANGES_REQUESTED" || pr.ReviewDecision == "REVIEW_REQUIRED" {
		add("review decision", "fail", pr.ReviewDecision)
	} else {
		add("review decision", "pass", emptyAs(pr.ReviewDecision, "n/a"))
	}
	if len(pr.StatusCheckRollup) == 0 {
		add("checks", "pass", "n/a")
		return
	}
	for _, check := range pr.StatusCheckRollup {
		if !checkRollupPassed(check) {
			add("checks", "fail", checkName(check)+" "+checkState(check))
			return
		}
	}
	add("checks", "pass", fmt.Sprintf("%d check(s)", len(pr.StatusCheckRollup)))
}

func (a *App) addReviewLaneReadyCheck(ctx context.Context, cfg config.Config, prNumber int, lane string, add func(string, string, string)) {
	if err := tmux.EnsureExists(); err != nil {
		add("review lane", "fail", err.Error())
		return
	}
	window := reviewLaneWindow(prNumber, lane)
	output, err := tmux.New().CapturePane(ctx, tmux.Target(cfg.Session, window))
	if err != nil {
		add("review lane", "fail", err.Error())
		return
	}
	if hasReviewPriorityFindings(output) {
		add("review lane", "fail", "P1/P2/P3 findings visible in "+window)
		return
	}
	add("review lane", "pass", window)
}

func reviewLaneWindow(prNumber int, lane string) string {
	lane = agent.SanitizeWindowName(lane)
	if strings.HasPrefix(lane, fmt.Sprintf("pr-review-%d-", prNumber)) {
		return lane
	}
	return agent.ReviewWindowName("pr-review", fmt.Sprintf("%d-%s", prNumber, lane))
}

var reviewPriorityRE = regexp.MustCompile(`(?i)(\[[[:space:]]*P[123][[:space:]]*\]|\bP[123]\b)`)
var reviewClearPassRE = regexp.MustCompile(`(?i)^\s*(there\s+(are|were|is)\s+)?no\s+(open\s+|remaining\s+)?P1\s*(/|,)?\s*P2\s*(/|,)?\s*(or\s+)?P3\b`)

func hasReviewPriorityFindings(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if reviewClearPassRE.MatchString(line) {
			continue
		}
		if reviewPriorityRE.MatchString(line) {
			return true
		}
	}
	return false
}

func checkRollupPassed(check ghx.CheckRollup) bool {
	state := strings.ToUpper(strings.TrimSpace(check.State))
	status := strings.ToUpper(strings.TrimSpace(check.Status))
	conclusion := strings.ToUpper(strings.TrimSpace(check.Conclusion))
	if state != "" {
		return state == "SUCCESS" || state == "SKIPPED" || state == "NEUTRAL"
	}
	if status != "" && status != "COMPLETED" {
		return false
	}
	if conclusion == "" {
		return status == "COMPLETED"
	}
	return conclusion == "SUCCESS" || conclusion == "SKIPPED" || conclusion == "NEUTRAL"
}

func checkName(check ghx.CheckRollup) string {
	if check.Name != "" {
		return check.Name
	}
	if check.Workflow != "" {
		return check.Workflow
	}
	return "check"
}

func checkState(check ghx.CheckRollup) string {
	for _, value := range []string{check.State, check.Conclusion, check.Status} {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "unknown"
}

func (a *App) renderReadyChecks(checks []readyCheck) error {
	tw := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "CHECK\tSTATUS\tDETAIL"); err != nil {
		return err
	}
	for _, check := range checks {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", check.Name, check.Status, check.Detail); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func readyChecksFailed(checks []readyCheck) bool {
	for _, check := range checks {
		if check.Status == "fail" {
			return true
		}
	}
	return false
}
