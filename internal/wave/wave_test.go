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
