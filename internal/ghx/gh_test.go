package ghx

import (
	"context"
	"reflect"
	"testing"

	"github.com/chenrui333/agent-yard/internal/execx"
)

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

type recordingRunner struct {
	command execx.Command
	result  execx.Result
	err     error
}

func (r *recordingRunner) Run(_ context.Context, command execx.Command) (execx.Result, error) {
	r.command = command
	return r.result, r.err
}
