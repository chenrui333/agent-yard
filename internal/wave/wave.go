package wave

import (
	"fmt"
	"strings"

	"github.com/chenrui333/agent-yard/internal/task"
)

type Options struct {
	Limit                       int
	EligibleStatuses            map[task.Status]bool
	PreferDistinctServiceFamily bool
}

type Selection struct {
	Task     task.Task
	Lane     string
	Reason   string
	Warnings []string
}

func SelectTasks(ledger task.Ledger, opts Options) []Selection {
	if opts.Limit <= 0 {
		return nil
	}
	eligible := opts.EligibleStatuses
	if len(eligible) == 0 {
		eligible = map[task.Status]bool{task.StatusReady: true}
	}
	selected := make([]Selection, 0, opts.Limit)
	selectedIDs := map[string]bool{}
	usedFamilies := map[string]bool{}

	add := func(item task.Task, reason string) bool {
		if len(selected) >= opts.Limit || selectedIDs[item.ID] || !eligible[item.Status] {
			return false
		}
		lane := item.AssignedAgent
		if lane == "" {
			lane = fmt.Sprintf("impl-%02d", len(selected)+1)
		}
		var warnings []string
		family := strings.TrimSpace(item.ServiceFamily)
		if family == "" {
			warnings = append(warnings, "missing service_family")
		} else if usedFamilies[family] {
			warnings = append(warnings, "service_family already selected")
		}
		selected = append(selected, Selection{Task: item, Lane: lane, Reason: reason, Warnings: warnings})
		selectedIDs[item.ID] = true
		if family != "" {
			usedFamilies[family] = true
		}
		return true
	}

	if opts.PreferDistinctServiceFamily {
		for _, item := range ledger.Tasks {
			family := strings.TrimSpace(item.ServiceFamily)
			if family == "" || usedFamilies[family] {
				continue
			}
			add(item, "distinct service_family")
		}
	}
	for _, item := range ledger.Tasks {
		add(item, "eligible fill")
	}
	return selected
}

func Eligible(statuses ...task.Status) map[task.Status]bool {
	out := make(map[task.Status]bool, len(statuses))
	for _, status := range statuses {
		out[status] = true
	}
	return out
}
