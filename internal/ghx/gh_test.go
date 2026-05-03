package ghx

import (
	"context"
	"reflect"
	"testing"

	"github.com/chenrui333/agent-yard/internal/execx"
)

func TestParsePRView(t *testing.T) {
	pr, err := ParsePRView(`{"number":42,"title":"feat: add thing","url":"https://github.com/o/r/pull/42","state":"OPEN","headRefName":"feature","baseRefName":"main","mergeStateStatus":"CLEAN","reviewDecision":"APPROVED","statusCheckRollup":[{"name":"test","status":"COMPLETED","conclusion":"SUCCESS"}]}`)
	if err != nil {
		t.Fatalf("ParsePRView returned error: %v", err)
	}
	if pr.Number != 42 || pr.HeadRefName != "feature" || pr.ReviewDecision != "APPROVED" {
		t.Fatalf("parsed PR = %#v", pr)
	}
	if len(pr.StatusCheckRollup) != 1 || pr.StatusCheckRollup[0].Conclusion != "SUCCESS" {
		t.Fatalf("checks = %#v", pr.StatusCheckRollup)
	}
}

func TestPRNumberFromURL(t *testing.T) {
	if got := PRNumberFromURL("https://github.com/o/r/pull/123"); got != 123 {
		t.Fatalf("PRNumberFromURL = %d; want 123", got)
	}
}

func TestAuthStatusScopesHost(t *testing.T) {
	runner := &recordingRunner{}
	client := Client{Runner: runner}
	if err := client.AuthStatus(context.Background(), "ghe.example.com"); err != nil {
		t.Fatalf("AuthStatus returned error: %v", err)
	}
	wantArgs := []string{"auth", "status", "--hostname", "ghe.example.com"}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("args = %#v; want %#v", runner.command.Args, wantArgs)
	}
	if runner.command.Dir != "" {
		t.Fatalf("dir = %q; want empty", runner.command.Dir)
	}
}

func TestPRForBranchUsesHeadFilter(t *testing.T) {
	runner := &recordingRunner{result: execx.Result{Stdout: `[{"number":7,"url":"https://github.com/o/r/pull/7","headRefName":"feature","baseRefName":"main","headRepositoryOwner":{"login":"owner"},"headRepository":{"name":"repo"},"isCrossRepository":false}]`}}
	client := Client{Runner: runner}
	pr, ok, err := client.PRForBranch(context.Background(), "/repo", "owner/repo", "feature", "main")
	if err != nil {
		t.Fatalf("PRForBranch returned error: %v", err)
	}
	if !ok || pr.Number != 7 {
		t.Fatalf("PRForBranch = %#v, %v; want PR 7", pr, ok)
	}
	wantArgs := []string{"pr", "list", "--head", "feature", "--state", "open", "--limit", "100", "--json", prJSONFields, "--repo", "owner/repo"}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("args = %#v; want %#v", runner.command.Args, wantArgs)
	}
}

func TestPRForBranchSelectsMatchingBaseAndHeadRepository(t *testing.T) {
	runner := &recordingRunner{result: execx.Result{Stdout: `[
{"number":1,"url":"https://github.com/o/r/pull/1","headRefName":"feature","baseRefName":"release","headRepositoryOwner":{"login":"o"},"headRepository":{"name":"r"},"isCrossRepository":false},
{"number":2,"url":"https://github.com/fork/r/pull/2","headRefName":"feature","baseRefName":"main","headRepositoryOwner":{"login":"fork"},"headRepository":{"name":"r"},"isCrossRepository":true},
{"number":3,"url":"https://github.com/o/r/pull/3","headRefName":"feature","baseRefName":"main","headRepositoryOwner":{"login":"o"},"headRepository":{"name":"r"},"isCrossRepository":false}
]`}}
	client := Client{Runner: runner}
	pr, ok, err := client.PRForBranch(context.Background(), "/repo", "o/r", "feature", "main")
	if err != nil {
		t.Fatalf("PRForBranch returned error: %v", err)
	}
	if !ok || pr.Number != 3 {
		t.Fatalf("PRForBranch = %#v, %v; want PR 3", pr, ok)
	}
}

func TestPRForBranchRejectsOnlyMismatchedPRs(t *testing.T) {
	runner := &recordingRunner{result: execx.Result{Stdout: `[{"number":1,"url":"https://github.com/o/r/pull/1","headRefName":"feature","baseRefName":"release","headRepositoryOwner":{"login":"o"},"headRepository":{"name":"r"},"isCrossRepository":false}]`}}
	client := Client{Runner: runner}
	_, ok, err := client.PRForBranch(context.Background(), "/repo", "o/r", "feature", "main")
	if err == nil {
		t.Fatal("PRForBranch returned nil error for mismatched PR")
	}
	if ok {
		t.Fatal("PRForBranch returned ok=true for mismatched PR")
	}
}

type recordingRunner struct {
	command execx.Command
	result  execx.Result
	err     error
}

func (r *recordingRunner) Run(_ context.Context, command execx.Command) (execx.Result, error) {
	r.command = command
	return r.result, r.err
}
