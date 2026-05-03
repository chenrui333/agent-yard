package wave

import (
	"fmt"
	"strings"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/task"
)

type Options struct {
	Limit                       int
	EligibleStatuses            map[task.Status]bool
	PreferDistinctServiceFamily bool
	ReservedLanes               map[string]string
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
	usedLanes := activeLanes(ledger)
	for lane, owner := range opts.ReservedLanes {
		lane = normalizeLane(lane)
		if lane == "" {
			continue
		}
		usedLanes[lane] = owner
	}

	add := func(item task.Task, reason string) bool {
		if len(selected) >= opts.Limit || selectedIDs[item.ID] || !eligible[item.Status] {
			return false
		}
		var warnings []string
		lane := normalizeLane(item.AssignedAgent)
		if lane == "" {
			lane = nextLane(len(selected)+1, usedLanes)
		} else if owner, used := usedLanes[lane]; used && owner != item.ID {
			lane = nextLane(len(selected)+1, usedLanes)
			warnings = append(warnings, "assigned_agent lane conflict; reassigned")
		}
		family := strings.TrimSpace(item.ServiceFamily)
		if family == "" {
			warnings = append(warnings, "missing service_family")
		} else if usedFamilies[family] {
			warnings = append(warnings, "service_family already selected")
		}
		selected = append(selected, Selection{Task: item, Lane: lane, Reason: reason, Warnings: warnings})
		selectedIDs[item.ID] = true
		usedLanes[lane] = item.ID
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

func activeLanes(ledger task.Ledger) map[string]string {
	type reservation struct {
		owner    string
		priority int
	}
	reservations := map[string]reservation{}
	for _, item := range ledger.Tasks {
		lane := normalizeLane(item.AssignedAgent)
		if lane == "" || !reservesLane(item.Status) {
			continue
		}
		priority := laneReservationPriority(item.Status)
		current, exists := reservations[lane]
		if !exists || priority > current.priority {
			reservations[lane] = reservation{owner: item.ID, priority: priority}
			continue
		}
		if current.owner != item.ID && priority == current.priority {
			reservations[lane] = reservation{owner: laneConflictOwner, priority: priority}
		}
	}
	used := map[string]string{}
	for lane, current := range reservations {
		used[lane] = current.owner
	}
	return used
}

const laneConflictOwner = "<conflict>"

func ReservedLanes(ledger task.Ledger) map[string]string {
	used := activeLanes(ledger)
	reserved := make(map[string]string, len(used))
	for lane, owner := range used {
		reserved[lane] = owner
	}
	return reserved
}

func reservesLane(status task.Status) bool {
	return status != task.StatusMerged && status != task.StatusBlocked
}

func laneReservationPriority(status task.Status) int {
	switch status {
	case task.StatusRunning, task.StatusNeedsReview, task.StatusPROpened, task.StatusReviewPending, task.StatusChangesRequested, task.StatusMergeReady:
		return 3
	case task.StatusClaimed, task.StatusWorktreeCreated:
		return 2
	case task.StatusReady:
		return 1
	default:
		return 0
	}
}

func normalizeLane(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return agent.SanitizeWindowName(value)
}

func nextLane(start int, used map[string]string) string {
	if start < 1 {
		start = 1
	}
	for i := start; ; i++ {
		lane := fmt.Sprintf("impl-%02d", i)
		if _, exists := used[lane]; !exists {
			return lane
		}
	}
}

func Eligible(statuses ...task.Status) map[task.Status]bool {
	out := make(map[task.Status]bool, len(statuses))
	for _, status := range statuses {
		out[status] = true
	}
	return out
}
