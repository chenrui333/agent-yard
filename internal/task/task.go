package task

import (
	"fmt"
	"strings"
)

type Status string

const (
	StatusReady            Status = "ready"
	StatusClaimed          Status = "claimed"
	StatusWorktreeCreated  Status = "worktree_created"
	StatusRunning          Status = "running"
	StatusNeedsReview      Status = "needs_review"
	StatusPROpened         Status = "pr_opened"
	StatusReviewPending    Status = "review_pending"
	StatusChangesRequested Status = "changes_requested"
	StatusMergeReady       Status = "merge_ready"
	StatusMerged           Status = "merged"
	StatusBlocked          Status = "blocked"
)

var ValidStatuses = map[Status]bool{
	StatusReady:            true,
	StatusClaimed:          true,
	StatusWorktreeCreated:  true,
	StatusRunning:          true,
	StatusNeedsReview:      true,
	StatusPROpened:         true,
	StatusReviewPending:    true,
	StatusChangesRequested: true,
	StatusMergeReady:       true,
	StatusMerged:           true,
	StatusBlocked:          true,
}

type Ledger struct {
	Tasks []Task `yaml:"tasks"`
}

type Task struct {
	ID            string `yaml:"id"`
	Issue         int    `yaml:"issue"`
	Checkbox      string `yaml:"checkbox"`
	ServiceFamily string `yaml:"service_family,omitempty"`
	Branch        string `yaml:"branch"`
	Worktree      string `yaml:"worktree"`
	Status        Status `yaml:"status"`
	AssignedAgent string `yaml:"assigned_agent,omitempty"`
	PRURL         string `yaml:"pr_url"`
	PRNumber      int    `yaml:"pr_number"`
}

func EmptyLedger() Ledger {
	return Ledger{Tasks: []Task{}}
}

func (l Ledger) Find(id string) (*Task, int, bool) {
	for i := range l.Tasks {
		if l.Tasks[i].ID == id {
			return &l.Tasks[i], i, true
		}
	}
	return nil, -1, false
}

func (l *Ledger) Update(id string, update func(*Task) error) error {
	task, _, ok := l.Find(id)
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	if err := update(task); err != nil {
		return err
	}
	return Validate(*l)
}

func Validate(l Ledger) error {
	ids := map[string]bool{}
	branches := map[string]string{}
	worktrees := map[string]string{}
	for _, item := range l.Tasks {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("task id is required")
		}
		if ids[item.ID] {
			return fmt.Errorf("duplicate task id %q", item.ID)
		}
		ids[item.ID] = true
		if item.Status == "" {
			item.Status = StatusReady
		}
		if !ValidStatuses[item.Status] {
			return fmt.Errorf("task %q has invalid status %q", item.ID, item.Status)
		}
		if item.Branch != "" {
			if owner := branches[item.Branch]; owner != "" {
				return fmt.Errorf("duplicate branch %q on tasks %q and %q", item.Branch, owner, item.ID)
			}
			branches[item.Branch] = item.ID
		}
		if item.Worktree != "" {
			if owner := worktrees[item.Worktree]; owner != "" {
				return fmt.Errorf("duplicate worktree %q on tasks %q and %q", item.Worktree, owner, item.ID)
			}
			worktrees[item.Worktree] = item.ID
		}
	}
	return nil
}

func Normalize(l *Ledger) {
	if l.Tasks == nil {
		l.Tasks = []Task{}
	}
	for i := range l.Tasks {
		if l.Tasks[i].Status == "" {
			l.Tasks[i].Status = StatusReady
		}
	}
}

func StatusList() []Status {
	return []Status{
		StatusReady,
		StatusClaimed,
		StatusWorktreeCreated,
		StatusRunning,
		StatusNeedsReview,
		StatusPROpened,
		StatusReviewPending,
		StatusChangesRequested,
		StatusMergeReady,
		StatusMerged,
		StatusBlocked,
	}
}
