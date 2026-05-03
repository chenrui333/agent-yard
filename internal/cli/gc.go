package cli

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

func (a *App) newGCCmd() *cobra.Command {
	var prune bool
	var merged bool
	var force bool
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Report cleanup candidates for yard run/review state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runGC(cmd, prune, merged, force)
		},
	}
	cmd.Flags().BoolVar(&prune, "prune", false, "remove safe cleanup candidates instead of only reporting them")
	cmd.Flags().BoolVar(&merged, "merged", false, "limit pruning to merged task and PR state")
	cmd.Flags().BoolVar(&force, "force", false, "force removal of dirty review worktrees")
	return cmd
}

func (a *App) runGC(cmd *cobra.Command, prune, merged, force bool) error {
	if prune {
		if !merged {
			return fmt.Errorf("--prune requires --merged")
		}
		return a.pruneMergedGC(cmd, force)
	}
	for _, dir := range []string{a.yardPath("runs"), a.yardPath("reviews")} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			a.printf("missing %s\n", dir)
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", dir, err)
		}
		a.printf("%s\n", dir)
		if len(entries) == 0 {
			a.printf("  no candidates\n")
			continue
		}
		for _, entry := range entries {
			a.printf("  %s\n", filepath.Join(dir, entry.Name()))
		}
	}
	a.printf("gc is report-only by default; use --prune --merged to remove safe merged-state candidates.\n")
	return nil
}

func (a *App) pruneMergedGC(cmd *cobra.Command, force bool) error {
	cfg, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	mergedTasks := map[string]bool{}
	mergedPRs := map[int]bool{}
	for _, item := range ledger.Tasks {
		if item.Status != task.StatusMerged {
			continue
		}
		mergedTasks[item.ID] = true
		if item.PRNumber != 0 {
			mergedPRs[item.PRNumber] = true
		} else if number := prNumberFromTaskURL(item); number != 0 {
			mergedPRs[number] = true
		}
	}
	removed := 0
	runsDir := a.yardPath("runs")
	for taskID := range mergedTasks {
		path, err := safeYardChild(runsDir, taskID)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		a.printf("removed %s\n", path)
		removed++
	}
	if err := a.pruneMergedReviewWorktrees(cmd, cfg, mergedPRs, force, &removed); err != nil {
		return err
	}
	a.printf("removed %d candidate(s)\n", removed)
	return nil
}

func (a *App) pruneMergedReviewWorktrees(cmd *cobra.Command, cfg config.Config, mergedPRs map[int]bool, force bool, removed *int) error {
	if len(mergedPRs) == 0 {
		return nil
	}
	reviewsDir := a.yardPath("reviews")
	entries, err := os.ReadDir(reviewsDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", reviewsDir, err)
	}
	git := gitx.New()
	repo := config.RepoPath(a.configPath, cfg)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		prNumber := reviewDirPRNumber(entry.Name())
		if prNumber == 0 || !mergedPRs[prNumber] {
			continue
		}
		path := filepath.Join(reviewsDir, entry.Name())
		if !force {
			dirty, err := git.IsDirty(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("check review worktree %s: %w", path, err)
			}
			if dirty {
				return fmt.Errorf("review worktree %s is dirty; rerun with --force to remove it", path)
			}
		}
		if err := git.RemoveWorktree(cmd.Context(), repo, path, force); err != nil {
			return err
		}
		a.printf("removed %s\n", path)
		*removed = *removed + 1
	}
	return nil
}

func reviewDirPRNumber(name string) int {
	if !strings.HasPrefix(name, "pr-") {
		return 0
	}
	rest := strings.TrimPrefix(name, "pr-")
	value, _, _ := strings.Cut(rest, "-")
	number, _ := strconv.Atoi(value)
	return number
}

func prNumberFromTaskURL(item task.Task) int {
	if item.PRURL == "" {
		return 0
	}
	parsed, err := url.Parse(item.PRURL)
	if err != nil {
		return 0
	}
	parts := strings.Split(strings.Trim(strings.TrimSpace(parsed.Path), "/"), "/")
	if len(parts) < 2 || parts[len(parts)-2] != "pull" {
		return 0
	}
	number, _ := strconv.Atoi(parts[len(parts)-1])
	return number
}

func safeYardChild(root, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("unsafe empty yard child path")
	}
	cleanName := filepath.Clean(name)
	if filepath.IsAbs(name) || cleanName != name || cleanName == "." || strings.ContainsAny(name, `/\\`) || strings.HasPrefix(cleanName, "..") {
		return "", fmt.Errorf("unsafe yard child path %q", name)
	}
	root = filepath.Clean(root)
	path := filepath.Join(root, cleanName)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe yard child path %q", name)
	}
	return path, nil
}
