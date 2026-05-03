package issue

import (
	"testing"

	"github.com/chenrui333/agent-yard/internal/task"
)

func TestParseCheckboxesTracksSections(t *testing.T) {
	body := `# Root

## Route 53

- [ ] Hosted zone support
- [x] Completed item

## S3 Buckets

* [ ] Bucket children
`
	boxes := ParseCheckboxes(body)
	if len(boxes) != 3 {
		t.Fatalf("len(ParseCheckboxes) = %d; want 3", len(boxes))
	}
	if boxes[0].Text != "Hosted zone support" || boxes[0].Section != "Route 53" || boxes[0].Checked {
		t.Fatalf("first checkbox = %#v", boxes[0])
	}
	if !boxes[1].Checked {
		t.Fatalf("second checkbox should be checked: %#v", boxes[1])
	}
	if boxes[2].Section != "S3 Buckets" || boxes[2].Slug != "bucket-children" {
		t.Fatalf("third checkbox = %#v", boxes[2])
	}
}

func TestImportTasksPreservesExistingAndSkipsChecked(t *testing.T) {
	existing := task.Ledger{Tasks: []task.Task{
		{ID: "route53-existing", Issue: 338, Checkbox: "Hosted zone support", ServiceFamily: "route-53", Branch: "route53-existing", Status: task.StatusRunning, PRNumber: 12},
	}}
	boxes := ParseCheckboxes(`## Route 53
- [ ] Hosted zone support
- [ ] Resolver support
- [x] Already done
`)

	result := ImportTasks(existing, boxes, ImportOptions{IssueNumber: 338, IDPrefix: "aws-", BranchPrefix: "work-", Section: "route-53"})
	if result.Added != 1 || result.Skipped != 2 {
		t.Fatalf("result Added=%d Skipped=%d; want 1/2", result.Added, result.Skipped)
	}
	if got := result.Tasks[0]; got.ID != "aws-resolver-support" || got.Branch != "work-resolver-support" || got.ServiceFamily != "route-53" || got.Status != task.StatusReady {
		t.Fatalf("imported task = %#v", got)
	}
}

func TestImportTasksUsesIssuePrefixByDefault(t *testing.T) {
	boxes := ParseCheckboxes("- [ ] Add CloudFront policies")
	result := ImportTasks(task.EmptyLedger(), boxes, ImportOptions{IssueNumber: 338})
	if len(result.Tasks) != 1 {
		t.Fatalf("len(result.Tasks) = %d; want 1", len(result.Tasks))
	}
	if result.Tasks[0].ID != "issue-338-add-cloudfront-policies" {
		t.Fatalf("ID = %q", result.Tasks[0].ID)
	}
	if result.Tasks[0].Branch != result.Tasks[0].ID {
		t.Fatalf("Branch = %q; want %q", result.Tasks[0].Branch, result.Tasks[0].ID)
	}
}

func TestImportTasksAvoidsBranchCollisions(t *testing.T) {
	existing := task.Ledger{Tasks: []task.Task{{ID: "existing", Branch: "issue-338-add-cloudfront-policies", Status: task.StatusReady}}}
	boxes := ParseCheckboxes("- [ ] Add CloudFront policies")
	result := ImportTasks(existing, boxes, ImportOptions{IssueNumber: 338})
	if got, want := result.Tasks[0].Branch, "issue-338-add-cloudfront-policies-2"; got != want {
		t.Fatalf("Branch = %q; want %q", got, want)
	}
}

func TestImportTasksDedupesWithinSectionOnly(t *testing.T) {
	boxes := ParseCheckboxes(`## Route 53
- [ ] Policy
## S3
- [ ] Policy
`)
	result := ImportTasks(task.EmptyLedger(), boxes, ImportOptions{IssueNumber: 338})
	if result.Added != 2 {
		t.Fatalf("Added = %d; want 2", result.Added)
	}
}

func TestImportTasksCountsLimitSkips(t *testing.T) {
	boxes := ParseCheckboxes(`- [ ] One
- [ ] Two
`)
	result := ImportTasks(task.EmptyLedger(), boxes, ImportOptions{IssueNumber: 338, Limit: 1})
	if result.Added != 1 || result.Skipped != 1 {
		t.Fatalf("Added/Skipped = %d/%d; want 1/1", result.Added, result.Skipped)
	}
}
