package cli

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/chenrui333/agent-yard/internal/execx"
	"github.com/chenrui333/agent-yard/internal/tmux"
)

func TestPlanTmuxWindowRejectsActivePaneForIdleReuse(t *testing.T) {
	runner := &tmuxWindowRunner{
		windows: "impl-01\n",
		panes:   "%1\tcodex\t0\t\n",
	}
	_, err := planTmuxWindow(context.Background(), tmux.Client{Runner: runner}, "yard", "impl-01", &launchOptions{reuseIdle: true})
	if err == nil || !strings.Contains(err.Error(), "running codex") {
		t.Fatalf("planTmuxWindow error = %v; want active pane rejection", err)
	}
}

func TestExecuteTmuxWindowReusesIdlePane(t *testing.T) {
	runner := &tmuxWindowRunner{
		windows: "impl-01\n",
		panes:   "%1\tzsh\t0\t\n",
	}
	client := tmux.Client{Runner: runner}
	plan, err := planTmuxWindow(context.Background(), client, "yard", "impl-01", &launchOptions{reuseIdle: true})
	if err != nil {
		t.Fatalf("planTmuxWindow returned error: %v", err)
	}
	if err := executeTmuxWindowPlan(context.Background(), client, plan, "codex exec prompt"); err != nil {
		t.Fatalf("executeTmuxWindowPlan returned error: %v", err)
	}
	assertTmuxCommands(t, runner.commands, [][]string{
		{"list-windows", "-t", "yard", "-F", "#{window_name}"},
		{"list-panes", "-t", "yard:impl-01", "-F", "#{pane_id}\t#{pane_current_command}\t#{pane_dead}\t#{pane_dead_status}"},
		{"send-keys", "-t", "yard:impl-01", "codex exec prompt", "C-m"},
	})
}

func TestExecuteTmuxWindowRespawnsDeadPane(t *testing.T) {
	runner := &tmuxWindowRunner{
		windows: "impl-01\n",
		panes:   "%1\t\t1\t0\n",
	}
	client := tmux.Client{Runner: runner}
	plan, err := planTmuxWindow(context.Background(), client, "yard", "impl-01", &launchOptions{reuseIdle: true})
	if err != nil {
		t.Fatalf("planTmuxWindow returned error: %v", err)
	}
	if err := executeTmuxWindowPlan(context.Background(), client, plan, "codex exec prompt"); err != nil {
		t.Fatalf("executeTmuxWindowPlan returned error: %v", err)
	}
	assertTmuxCommands(t, runner.commands, [][]string{
		{"list-windows", "-t", "yard", "-F", "#{window_name}"},
		{"list-panes", "-t", "yard:impl-01", "-F", "#{pane_id}\t#{pane_current_command}\t#{pane_dead}\t#{pane_dead_status}"},
		{"respawn-pane", "-k", "-t", "yard:impl-01", "codex exec prompt"},
	})
}

func TestExecuteTmuxWindowReplacesExistingWindow(t *testing.T) {
	runner := &tmuxWindowRunner{windows: "impl-01\n"}
	client := tmux.Client{Runner: runner}
	plan, err := planTmuxWindow(context.Background(), client, "yard", "impl-01", &launchOptions{replaceWindow: true})
	if err != nil {
		t.Fatalf("planTmuxWindow returned error: %v", err)
	}
	if err := executeTmuxWindowPlan(context.Background(), client, plan, "codex exec prompt"); err != nil {
		t.Fatalf("executeTmuxWindowPlan returned error: %v", err)
	}
	assertTmuxCommands(t, runner.commands, [][]string{
		{"list-windows", "-t", "yard", "-F", "#{window_name}"},
		{"kill-window", "-t", "yard:impl-01"},
		{"new-window", "-t", "yard", "-n", "impl-01"},
		{"send-keys", "-t", "yard:impl-01", "codex exec prompt", "C-m"},
	})
}

type tmuxWindowRunner struct {
	windows  string
	panes    string
	commands [][]string
}

func (r *tmuxWindowRunner) Run(_ context.Context, command execx.Command) (execx.Result, error) {
	r.commands = append(r.commands, append([]string{}, command.Args...))
	switch command.Args[0] {
	case "list-windows":
		return execx.Result{Stdout: r.windows}, nil
	case "list-panes":
		return execx.Result{Stdout: r.panes}, nil
	default:
		return execx.Result{}, nil
	}
}

func assertTmuxCommands(t *testing.T, got, want [][]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tmux commands = %#v; want %#v", got, want)
	}
}
