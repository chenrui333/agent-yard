package gitx

import (
	"context"
	"reflect"
	"testing"

	"github.com/chenrui333/agent-yard/internal/execx"
)

func TestParseWorktreeList(t *testing.T) {
	input := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo.feature\nHEAD def456\nbranch refs/heads/feature\n\nworktree /detached\nHEAD fedcba\ndetached\n"
	got := ParseWorktreeList(input)
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	if got[0].Path != "/repo" || got[0].Branch != "main" {
		t.Fatalf("first worktree = %#v", got[0])
	}
	if got[1].Path != "/repo.feature" || got[1].Branch != "feature" {
		t.Fatalf("second worktree = %#v", got[1])
	}
	if !got[2].Detached {
		t.Fatalf("third worktree detached = false: %#v", got[2])
	}
}

func TestParseAheadBehind(t *testing.T) {
	got, err := ParseAheadBehind("2\t5\n")
	if err != nil {
		t.Fatalf("ParseAheadBehind returned error: %v", err)
	}
	if got.Behind != 2 || got.Ahead != 5 {
		t.Fatalf("AheadBehind = %#v; want behind=2 ahead=5", got)
	}
	if _, err := ParseAheadBehind("oops"); err == nil {
		t.Fatal("ParseAheadBehind returned nil for invalid output")
	}
}

func TestRemoteBranchExistsUsesExactRef(t *testing.T) {
	runner := &recordingRunner{result: execx.Result{Stdout: "abc\trefs/heads/feature\n"}}
	client := Client{Runner: runner}
	exists, err := client.RemoteBranchExists(context.Background(), "/repo", "origin", "feature")
	if err != nil {
		t.Fatalf("RemoteBranchExists returned error: %v", err)
	}
	if !exists {
		t.Fatal("RemoteBranchExists = false; want true")
	}
	wantArgs := []string{"ls-remote", "origin", "refs/heads/feature"}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("args = %#v; want %#v", runner.command.Args, wantArgs)
	}
}

func TestRemoteTrackingBranchExistsUsesLocalRef(t *testing.T) {
	runner := &recordingRunner{}
	client := Client{Runner: runner}
	exists, err := client.RemoteTrackingBranchExists(context.Background(), "/repo", "origin", "feature")
	if err != nil {
		t.Fatalf("RemoteTrackingBranchExists returned error: %v", err)
	}
	if !exists {
		t.Fatal("RemoteTrackingBranchExists = false; want true")
	}
	wantArgs := []string{"show-ref", "--verify", "--quiet", "refs/remotes/origin/feature"}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("args = %#v; want %#v", runner.command.Args, wantArgs)
	}
}

func TestRemoteTrackingBranchExistsTreatsMissingRefAsFalse(t *testing.T) {
	runner := &recordingRunner{err: &execx.CommandError{Result: execx.Result{ExitCode: 1}}}
	client := Client{Runner: runner}
	exists, err := client.RemoteTrackingBranchExists(context.Background(), "/repo", "origin", "missing")
	if err != nil {
		t.Fatalf("RemoteTrackingBranchExists returned error: %v", err)
	}
	if exists {
		t.Fatal("RemoteTrackingBranchExists = true; want false")
	}
}

func TestDiffCheckSinceUsesMergeBaseRange(t *testing.T) {
	runner := &sequenceRunner{results: []execx.Result{
		{Stdout: "abc123\n"},
		{},
	}}
	client := Client{Runner: runner}
	if err := client.DiffCheckSince(context.Background(), "/repo", "origin/main"); err != nil {
		t.Fatalf("DiffCheckSince returned error: %v", err)
	}
	want := [][]string{
		{"merge-base", "HEAD", "origin/main"},
		{"diff", "--check", "abc123..HEAD"},
	}
	if len(runner.commands) != len(want) {
		t.Fatalf("recorded %d command(s); want %d", len(runner.commands), len(want))
	}
	for i := range want {
		if runner.commands[i].Dir != "/repo" {
			t.Fatalf("command %d dir = %q; want /repo", i, runner.commands[i].Dir)
		}
		if !reflect.DeepEqual(runner.commands[i].Args, want[i]) {
			t.Fatalf("command %d args = %#v; want %#v", i, runner.commands[i].Args, want[i])
		}
	}
}

func TestIsAncestorTreatsExitOneAsFalse(t *testing.T) {
	runner := &recordingRunner{err: &execx.CommandError{Result: execx.Result{ExitCode: 1}}}
	client := Client{Runner: runner}
	exists, err := client.IsAncestor(context.Background(), "/repo", "HEAD", "origin/feature")
	if err != nil {
		t.Fatalf("IsAncestor returned error: %v", err)
	}
	if exists {
		t.Fatal("IsAncestor = true; want false")
	}
	want := []string{"merge-base", "--is-ancestor", "HEAD", "origin/feature"}
	if !reflect.DeepEqual(runner.command.Args, want) {
		t.Fatalf("args = %#v; want %#v", runner.command.Args, want)
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

type sequenceRunner struct {
	commands []execx.Command
	results  []execx.Result
	errs     []error
}

func (r *sequenceRunner) Run(_ context.Context, command execx.Command) (execx.Result, error) {
	r.commands = append(r.commands, command)
	index := len(r.commands) - 1
	var result execx.Result
	if index < len(r.results) {
		result = r.results[index]
	}
	var err error
	if index < len(r.errs) {
		err = r.errs[index]
	}
	return result, err
}
