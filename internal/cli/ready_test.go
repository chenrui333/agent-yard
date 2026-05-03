package cli

import (
	"testing"

	"github.com/chenrui333/agent-yard/internal/ghx"
)

func TestHasReviewPriorityFindingsIgnoresClearPassMessage(t *testing.T) {
	if hasReviewPriorityFindings("There are no P1/P2/P3 TODO comments.") {
		t.Fatal("clear pass message should not count as TODO finding")
	}
	if !hasReviewPriorityFindings("- [P2] fix the race") {
		t.Fatal("P2 TODO finding was not detected")
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
