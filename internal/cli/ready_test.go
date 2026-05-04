package cli

import (
	"testing"

	"github.com/chenrui333/agent-yard/internal/ghx"
)

func TestHasReviewPriorityFindingsIgnoresClearPassMessage(t *testing.T) {
	clearMessages := []string{
		"There are no P1/P2/P3 TODO comments.",
		"- No P1/P2/P3 TODO comments.",
		"✅ No P1/P2/P3 TODO comments.",
	}
	for _, message := range clearMessages {
		if hasReviewPriorityFindings(message) {
			t.Fatalf("%q should not count as TODO finding", message)
		}
	}
	if !hasReviewPriorityFindings("- [P2] fix the race") {
		t.Fatal("P2 TODO finding was not detected")
	}
	if !hasReviewPriorityFindings("- [P3] No P1/P2/P3 gate exists for this command") {
		t.Fatal("P3 finding with negation text was not detected")
	}
}

func TestBlockingReviewPriorities(t *testing.T) {
	got := blockingReviewPriorities([]string{"p2", "P4", "P1", "P2", "note", "p3"})
	want := []string{"P2", "P1", "P3"}
	if len(got) != len(want) {
		t.Fatalf("blockingReviewPriorities length = %d; want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("blockingReviewPriorities[%d] = %q; want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestReviewLaneWindowAcceptsLaneOrFullWindow(t *testing.T) {
	if got, want := reviewLaneWindow(12, "pr-review-a"), "pr-review-12-pr-review-a"; got != want {
		t.Fatalf("reviewLaneWindow lane = %q; want %q", got, want)
	}
	if got, want := reviewLaneWindow(12, "pr-review-12-pr-review-b"), "pr-review-12-pr-review-b"; got != want {
		t.Fatalf("reviewLaneWindow window = %q; want %q", got, want)
	}
}

func TestCheckRollupPassed(t *testing.T) {
	passing := []ghx.CheckRollup{
		{State: "SUCCESS"},
		{Status: "COMPLETED", Conclusion: "SKIPPED"},
		{Status: "COMPLETED", Conclusion: "NEUTRAL"},
	}
	for _, check := range passing {
		if !checkRollupPassed(check) {
			t.Fatalf("checkRollupPassed(%#v) = false", check)
		}
	}
	failing := []ghx.CheckRollup{
		{State: "FAILURE"},
		{Status: "IN_PROGRESS"},
		{Status: "COMPLETED", Conclusion: "FAILURE"},
	}
	for _, check := range failing {
		if checkRollupPassed(check) {
			t.Fatalf("checkRollupPassed(%#v) = true", check)
		}
	}
}

func TestAddPRReadyChecksFailsReviewRequired(t *testing.T) {
	var checks []readyCheck
	addPRReadyChecks(ghx.PullRequest{State: "OPEN", MergeStateStatus: "CLEAN", ReviewDecision: "REVIEW_REQUIRED"}, func(name, status, detail string) {
		checks = append(checks, readyCheck{Name: name, Status: status, Detail: detail})
	})
	for _, check := range checks {
		if check.Name == "review decision" {
			if check.Status != "fail" {
				t.Fatalf("review decision status = %q; want fail", check.Status)
			}
			return
		}
	}
	t.Fatal("review decision check missing")
}
