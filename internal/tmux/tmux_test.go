package tmux

import (
	"context"
	"reflect"
	"testing"

	"github.com/chenrui333/agent-yard/internal/execx"
)

func TestParsePaneList(t *testing.T) {
	got := ParsePaneList("%1\tcodex\t0\t\n%2\tzsh\t1\t2\n")
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	if got[0].ID != "%1" || got[0].CurrentCommand != "codex" || got[0].Dead {
		t.Fatalf("first pane = %#v", got[0])
	}
	if got[1].ID != "%2" || !got[1].Dead || got[1].DeadStatus != "2" {
		t.Fatalf("second pane = %#v", got[1])
	}
}

func TestCapturePaneTailTrimsCapturedOutput(t *testing.T) {
	runner := &recordingRunner{result: execx.Result{Stdout: "one\ntwo\nthree\n"}}
	client := Client{Runner: runner}
	got, err := client.CapturePaneTail(context.Background(), "yard:impl-01", 1)
	if err != nil {
		t.Fatalf("CapturePaneTail returned error: %v", err)
	}
	if got != "three\n" {
		t.Fatalf("CapturePaneTail = %q; want %q", got, "three\n")
	}
	wantArgs := []string{"capture-pane", "-p", "-S", "-1", "-t", "yard:impl-01"}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("args = %#v; want %#v", runner.command.Args, wantArgs)
	}
}

func TestTailLinesPreservesUnterminatedLastLine(t *testing.T) {
	got := tailLines("one\ntwo\nthree", 2)
	if got != "two\nthree" {
		t.Fatalf("tailLines = %q; want %q", got, "two\nthree")
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
