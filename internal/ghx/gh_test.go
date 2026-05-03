package ghx

import "testing"

func TestParsePRView(t *testing.T) {
	pr, err := ParsePRView(`{"number":42,"title":"feat: add thing","url":"https://github.com/o/r/pull/42","state":"OPEN","headRefName":"feature","baseRefName":"main","mergeStateStatus":"CLEAN","reviewDecision":"APPROVED"}`)
	if err != nil {
		t.Fatalf("ParsePRView returned error: %v", err)
	}
	if pr.Number != 42 || pr.HeadRefName != "feature" || pr.ReviewDecision != "APPROVED" {
		t.Fatalf("parsed PR = %#v", pr)
	}
}

func TestPRNumberFromURL(t *testing.T) {
	if got := PRNumberFromURL("https://github.com/o/r/pull/123"); got != 123 {
		t.Fatalf("PRNumberFromURL = %d; want 123", got)
	}
}
