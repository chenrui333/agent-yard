package wave

import (
	"testing"

	"github.com/chenrui333/agent-yard/internal/task"
)

func TestSelectTasksPrefersDistinctServiceFamilies(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "ec2-a", ServiceFamily: "ec2", Status: task.StatusReady},
		{ID: "ec2-b", ServiceFamily: "ec2", Status: task.StatusReady},
		{ID: "s3", ServiceFamily: "s3", Status: task.StatusReady},
		{ID: "done", ServiceFamily: "route53", Status: task.StatusMerged},
		{ID: "route53", ServiceFamily: "route53", Status: task.StatusReady},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       3,
		EligibleStatuses:            Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	want := []string{"ec2-a", "s3", "route53"}
	if len(got) != len(want) {
		t.Fatalf("len = %d; want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Task.ID != want[i] {
			t.Fatalf("selection[%d] = %q; want %q", i, got[i].Task.ID, want[i])
		}
	}
	if got[0].Lane != "impl-01" || got[2].Lane != "impl-03" {
		t.Fatalf("lanes = %#v", got)
	}
}

func TestSelectTasksFillsWhenFamiliesRepeat(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "ec2-a", ServiceFamily: "ec2", Status: task.StatusReady},
		{ID: "ec2-b", ServiceFamily: "ec2", Status: task.StatusReady},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       2,
		EligibleStatuses:            Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	if got[1].Task.ID != "ec2-b" || len(got[1].Warnings) == 0 {
		t.Fatalf("second selection = %#v; want repeated-family warning", got[1])
	}
}

func TestSelectTasksAvoidsExistingAssignedLane(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "running", ServiceFamily: "ec2", Status: task.StatusRunning, AssignedAgent: "impl-01"},
		{ID: "ready", ServiceFamily: "s3", Status: task.StatusReady},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       1,
		EligibleStatuses:            Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Lane != "impl-02" {
		t.Fatalf("lane = %q; want impl-02", got[0].Lane)
	}
}

func TestSelectTasksAvoidsDuplicateSelectedLanes(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "preassigned", ServiceFamily: "ec2", Status: task.StatusReady, AssignedAgent: "impl-02"},
		{ID: "unassigned", ServiceFamily: "s3", Status: task.StatusReady},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       2,
		EligibleStatuses:            Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	if got[0].Lane != "impl-02" {
		t.Fatalf("first lane = %q; want impl-02", got[0].Lane)
	}
	if got[1].Lane == "impl-02" {
		t.Fatalf("second lane reused impl-02: %#v", got)
	}
}

func TestSelectTasksReassignsConflictingTaskLane(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "running", ServiceFamily: "ec2", Status: task.StatusRunning, AssignedAgent: "impl-01"},
		{ID: "ready", ServiceFamily: "s3", Status: task.StatusReady, AssignedAgent: "impl-01"},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       1,
		EligibleStatuses:            Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Task.ID != "ready" {
		t.Fatalf("task = %q; want ready", got[0].Task.ID)
	}
	if got[0].Lane != "impl-02" {
		t.Fatalf("lane = %q; want impl-02", got[0].Lane)
	}
	if len(got[0].Warnings) == 0 {
		t.Fatalf("warnings = %#v; want lane conflict warning", got[0].Warnings)
	}
}

func TestSelectTasksNormalizesAssignedLaneConflicts(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "running", ServiceFamily: "ec2", Status: task.StatusRunning, AssignedAgent: "impl 01"},
		{ID: "ready", ServiceFamily: "s3", Status: task.StatusReady},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       1,
		EligibleStatuses:            Eligible(task.StatusReady),
		PreferDistinctServiceFamily: true,
	})
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Lane != "impl-02" {
		t.Fatalf("lane = %q; want impl-02", got[0].Lane)
	}
}

func TestSelectTasksUsesExternalReservedLanes(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "ready", ServiceFamily: "s3", Status: task.StatusWorktreeCreated},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       1,
		EligibleStatuses:            Eligible(task.StatusWorktreeCreated),
		PreferDistinctServiceFamily: true,
		ReservedLanes:               map[string]string{"impl-01": "running"},
	})
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Lane != "impl-02" {
		t.Fatalf("lane = %q; want impl-02", got[0].Lane)
	}
}

func TestSelectTasksHonorsReservedLaneOverLaunchableOwner(t *testing.T) {
	ledger := task.Ledger{Tasks: []task.Task{
		{ID: "launchable", ServiceFamily: "s3", Status: task.StatusWorktreeCreated, AssignedAgent: "impl-01"},
	}}
	got := SelectTasks(ledger, Options{
		Limit:                       1,
		EligibleStatuses:            Eligible(task.StatusWorktreeCreated),
		PreferDistinctServiceFamily: true,
		ReservedLanes:               map[string]string{"impl-01": "running"},
	})
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Lane != "impl-02" {
		t.Fatalf("lane = %q; want impl-02", got[0].Lane)
	}
	if len(got[0].Warnings) == 0 {
		t.Fatalf("warnings = %#v; want lane conflict warning", got[0].Warnings)
	}
}
