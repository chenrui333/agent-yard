package cli

import (
	"path/filepath"
	"testing"

	"github.com/chenrui333/agent-yard/internal/task"
)

func TestPRNumberFromTaskURLRequiresPullSegment(t *testing.T) {
	if got := prNumberFromTaskURL(task.Task{PRURL: "https://github.com/o/r/pull/123"}); got != 123 {
		t.Fatalf("prNumberFromTaskURL = %d; want 123", got)
	}
	for _, value := range []string{
		"https://github.com/o/r/issues/123",
		"https://github.com/o/r/actions/runs/123",
		"https://github.com/o/r/pull/not-a-number",
		"https://github.com/o/r/pull/0",
		"owner/repo/pull/123",
	} {
		if got := prNumberFromTaskURL(task.Task{PRURL: value}); got != 0 {
			t.Fatalf("prNumberFromTaskURL(%q) = %d; want 0", value, got)
		}
	}
}

func TestSafeYardChildRejectsTraversal(t *testing.T) {
	root := filepath.Join("base", ".yard", "runs")
	if _, err := safeYardChild(root, "../outside"); err == nil {
		t.Fatal("safeYardChild accepted traversal")
	}
	if _, err := safeYardChild(root, filepath.Join("nested", "task")); err == nil {
		t.Fatal("safeYardChild accepted nested path")
	}
	got, err := safeYardChild(root, "task-1")
	if err != nil {
		t.Fatalf("safeYardChild returned error: %v", err)
	}
	if want := filepath.Join(root, "task-1"); got != want {
		t.Fatalf("safeYardChild = %q; want %q", got, want)
	}
}
