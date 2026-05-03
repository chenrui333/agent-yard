package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsDuplicates(t *testing.T) {
	ledger := Ledger{Tasks: []Task{
		{ID: "one", Branch: "branch", Worktree: "wt1", Status: StatusReady},
		{ID: "one", Branch: "branch2", Worktree: "wt2", Status: StatusReady},
	}}
	if err := Validate(ledger); err == nil {
		t.Fatal("Validate returned nil for duplicate task ID")
	}
}

func TestValidateRejectsDuplicateBranch(t *testing.T) {
	ledger := Ledger{Tasks: []Task{
		{ID: "one", Branch: "branch", Status: StatusReady},
		{ID: "two", Branch: "branch", Status: StatusReady},
	}}
	if err := Validate(ledger); err == nil {
		t.Fatal("Validate returned nil for duplicate branch")
	}
}

func TestLedgerUpdate(t *testing.T) {
	ledger := Ledger{Tasks: []Task{{ID: "route53", Status: StatusReady}}}
	if err := ledger.Update("route53", func(item *Task) error {
		item.Status = StatusRunning
		item.AssignedAgent = "impl-01"
		return nil
	}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	got, _, ok := ledger.Find("route53")
	if !ok {
		t.Fatal("task not found after update")
	}
	if got.Status != StatusRunning || got.AssignedAgent != "impl-01" {
		t.Fatalf("updated task = %#v", got)
	}
}

func TestStoreSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.yaml")
	store := NewStore(path)
	ledger := Ledger{Tasks: []Task{{
		ID:       "aws-route53",
		Issue:    338,
		Checkbox: "Route53 resources",
		Branch:   "aws-route53-resources",
		Worktree: "../terraformer.aws-route53-resources",
		Status:   StatusReady,
	}}}
	if err := store.Save(ledger); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("lock file was not removed, stat err = %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Tasks) != 1 || loaded.Tasks[0].ID != "aws-route53" {
		t.Fatalf("loaded ledger = %#v", loaded)
	}
}
