package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

func (a *App) newAttachCmd() *cobra.Command {
	var session string
	cmd := &cobra.Command{
		Use:   "attach [task-id]",
		Short: "Attach to the configured tmux session or a task window",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAttach(cmd, session, args)
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "tmux session override")
	return cmd
}

func (a *App) newCaptureCmd() *cobra.Command {
	var session string
	var tail int
	cmd := &cobra.Command{
		Use:   "capture <task-id>",
		Short: "Print the current tmux pane contents for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runCapture(cmd, session, args[0], tail)
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "tmux session override")
	cmd.Flags().IntVar(&tail, "tail", 0, "only print the last N captured pane lines")
	return cmd
}

func (a *App) newLanesCmd() *cobra.Command {
	var session string
	cmd := &cobra.Command{
		Use:   "lanes",
		Short: "Show live tmux lanes and their task mapping",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runLanes(cmd, session)
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "tmux session override")
	return cmd
}

func (a *App) runAttach(cmd *cobra.Command, session string, args []string) error {
	cfg, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	if session == "" {
		session = cfg.Session
	}
	target := session
	if len(args) == 1 {
		item, _, ok := ledger.Find(args[0])
		if !ok {
			return fmt.Errorf("task %q not found", args[0])
		}
		target = tmux.Target(session, agent.TaskWindowName(*item))
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	return tmux.Attach(cmd.Context(), target)
}

func (a *App) runCapture(cmd *cobra.Command, session, taskID string, tail int) error {
	cfg, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	if session == "" {
		session = cfg.Session
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	target := tmux.Target(session, agent.TaskWindowName(*item))
	out, err := tmux.New().CapturePaneTail(cmd.Context(), target, tail)
	if err != nil {
		return err
	}
	a.printf("%s", out)
	return nil
}

func (a *App) runLanes(cmd *cobra.Command, session string) error {
	cfg, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	if session == "" {
		session = cfg.Session
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	tmuxClient := tmux.New()
	exists, err := tmuxClient.HasSession(cmd.Context(), session)
	if err != nil {
		return err
	}
	if !exists {
		a.printf("session %s missing\n", session)
		return nil
	}
	windows, err := tmuxClient.ListWindows(cmd.Context(), session)
	if err != nil {
		return err
	}
	sort.Strings(windows)
	owners := laneOwners(ledger)
	seen := map[string]bool{}
	tw := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "WINDOW\tTASK\tSTATUS\tPANE"); err != nil {
		return err
	}
	for _, window := range windows {
		seen[window] = true
		pane := "unknown"
		if panes, err := tmuxClient.ListPanes(cmd.Context(), tmux.Target(session, window)); err == nil {
			pane = paneStatus(panes)
		}
		items := owners[window]
		if len(items) == 0 {
			items = prReviewWindowOwners(ledger, window)
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", window, ownerIDs(items), ownerStatuses(items), pane); err != nil {
			return err
		}
	}
	ownerWindows := make([]string, 0, len(owners))
	for window := range owners {
		ownerWindows = append(ownerWindows, window)
	}
	sort.Strings(ownerWindows)
	for _, window := range ownerWindows {
		items := owners[window]
		if seen[window] || !hasActiveLaneOwner(items) {
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\tmissing\n", window, ownerIDs(items), ownerStatuses(items)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func laneOwners(ledger task.Ledger) map[string][]task.Task {
	owners := map[string][]task.Task{}
	for _, item := range ledger.Tasks {
		owners[agent.TaskWindowName(item)] = append(owners[agent.TaskWindowName(item)], item)
	}
	return owners
}

func prReviewWindowOwners(ledger task.Ledger, window string) []task.Task {
	prNumber := prReviewWindowPRNumber(window)
	if prNumber == 0 {
		return nil
	}
	var owners []task.Task
	for _, item := range ledger.Tasks {
		if item.PRNumber == prNumber || prNumberFromTaskURL(item) == prNumber {
			owners = append(owners, item)
		}
	}
	return owners
}

func prReviewWindowPRNumber(window string) int {
	if !strings.HasPrefix(window, "pr-review-") {
		return 0
	}
	rest := strings.TrimPrefix(window, "pr-review-")
	value, _, _ := strings.Cut(rest, "-")
	number, _ := strconv.Atoi(value)
	return number
}

func ownerIDs(items []task.Task) string {
	if len(items) == 0 {
		return "-"
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return strings.Join(ids, ",")
}

func ownerStatuses(items []task.Task) string {
	if len(items) == 0 {
		return "-"
	}
	statuses := make([]string, 0, len(items))
	for _, item := range items {
		statuses = append(statuses, string(item.Status))
	}
	return strings.Join(statuses, ",")
}

func hasActiveLaneOwner(items []task.Task) bool {
	for _, item := range items {
		switch item.Status {
		case task.StatusRunning, task.StatusReviewPending, task.StatusChangesRequested:
			return true
		}
	}
	return false
}
