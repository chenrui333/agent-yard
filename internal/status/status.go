package status

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/chenrui333/agent-yard/internal/task"
)

type Row struct {
	TaskID       string
	LedgerStatus task.Status
	Branch       string
	Worktree     string
	WorktreeOK   bool
	Dirty        string
	AheadBehind  string
	ChangedFiles string
	Remote       string
	Tmux         string
	PR           string
	CIReview     string
}

func RenderSummary(w io.Writer, rows []Row) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TASK\tSTATUS\tBRANCH\tWORKTREE\tDIRTY\tA/B\tFILES\tREMOTE\tTMUX\tPR\tCI/REVIEW")
	for _, row := range rows {
		worktree := "missing"
		if row.WorktreeOK {
			worktree = "exists"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.TaskID, row.LedgerStatus, row.Branch, worktree, row.Dirty, row.AheadBehind, row.ChangedFiles, row.Remote, row.Tmux, row.PR, row.CIReview)
	}
	_ = tw.Flush()
}

func RenderBoard(w io.Writer, rows []Row) {
	byStatus := map[task.Status][]Row{}
	for _, row := range rows {
		byStatus[row.LedgerStatus] = append(byStatus[row.LedgerStatus], row)
	}
	for _, status := range task.StatusList() {
		group := byStatus[status]
		if len(group) == 0 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].TaskID < group[j].TaskID })
		fmt.Fprintf(w, "\n%s\n", status)
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "TASK\tBRANCH\tWORKTREE\tTMUX\tPR")
		for _, row := range group {
			worktree := "missing"
			if row.WorktreeOK {
				worktree = "exists"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", row.TaskID, row.Branch, worktree, row.Tmux, row.PR)
		}
		_ = tw.Flush()
	}
}
