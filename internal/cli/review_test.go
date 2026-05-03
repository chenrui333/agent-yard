package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenrui333/agent-yard/internal/task"
)

func TestPRCheckoutPreviewIncludesRepoArg(t *testing.T) {
	got := prCheckoutPreview("owner/repo", 123)
	want := "gh pr checkout 123 --detach --repo owner/repo"
	if got != want {
		t.Fatalf("prCheckoutPreview = %q; want %q", got, want)
	}
}

func TestReserveWaveTaskRejectsLaneReservedByActiveTask(t *testing.T) {
	store := task.NewStore(filepath.Join(t.TempDir(), "tasks.yaml"))
	if err := store.Save(task.Ledger{Tasks: []task.Task{
		{ID: "ready", Status: task.StatusReady, Branch: "ready"},
		{ID: "running", Status: task.StatusRunning, Branch: "running", AssignedAgent: "impl-01"},
	}}); err != nil {
		t.Fatalf("save ledger: %v", err)
	}

	app := &App{}
	_, err := app.reserveWaveTask(store, task.Task{ID: "ready", Status: task.StatusReady}, "impl-01")
	if err == nil || !strings.Contains(err.Error(), "lane impl-01 is reserved by running") {
		t.Fatalf("reserveWaveTask error = %v; want reserved lane error", err)
	}

	ledger, err := store.Load()
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	item, _, ok := ledger.Find("ready")
	if !ok {
		t.Fatal("ready task missing")
	}
	if item.Status != task.StatusReady || item.AssignedAgent != "" {
		t.Fatalf("ready task mutated after failed reserve: %#v", item)
	}
}
