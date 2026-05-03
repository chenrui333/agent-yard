package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chenrui333/agent-yard/internal/execx"
)

type Runner interface {
	Run(context.Context, execx.Command) (execx.Result, error)
}

type Client struct {
	Runner Runner
}

type Pane struct {
	ID             string
	CurrentCommand string
	Dead           bool
	DeadStatus     string
}

func New() Client {
	return Client{Runner: execx.Runner{}}
}

func EnsureExists() error {
	_, err := execx.LookPath("tmux")
	return err
}

func (c Client) run(ctx context.Context, args ...string) (execx.Result, error) {
	return c.Runner.Run(ctx, execx.Command{Name: "tmux", Args: args})
}

func (c Client) HasSession(ctx context.Context, session string) (bool, error) {
	_, err := c.run(ctx, "has-session", "-t", session)
	if err == nil {
		return true, nil
	}
	var cmdErr *execx.CommandError
	if errors.As(err, &cmdErr) && cmdErr.Result.ExitCode != 127 {
		return false, nil
	}
	return false, err
}

func (c Client) EnsureSession(ctx context.Context, session string) error {
	exists, err := c.HasSession(ctx, session)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = c.run(ctx, "new-session", "-d", "-s", session, "-n", "board")
	return err
}

func (c Client) NewWindow(ctx context.Context, session, name string) error {
	_, err := c.run(ctx, "new-window", "-t", session, "-n", name)
	return err
}

func (c Client) SendKeys(ctx context.Context, target, command string) error {
	_, err := c.run(ctx, "send-keys", "-t", target, command, "C-m")
	return err
}

func (c Client) CapturePane(ctx context.Context, target string) (string, error) {
	result, err := c.run(ctx, "capture-pane", "-p", "-t", target)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func (c Client) ListPanes(ctx context.Context, target string) ([]Pane, error) {
	result, err := c.run(ctx, "list-panes", "-t", target, "-F", "#{pane_id}\t#{pane_current_command}\t#{pane_dead}\t#{pane_dead_status}")
	if err != nil {
		return nil, err
	}
	return ParsePaneList(result.Stdout), nil
}

func Attach(ctx context.Context, target string) error {
	args := []string{"attach-session", "-t", target}
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run tmux %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (c Client) ListSessions(ctx context.Context) ([]string, error) {
	result, err := c.run(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, err
	}
	return splitLines(result.Stdout), nil
}

func (c Client) ListWindows(ctx context.Context, session string) ([]string, error) {
	result, err := c.run(ctx, "list-windows", "-t", session, "-F", "#{window_name}")
	if err != nil {
		return nil, err
	}
	return splitLines(result.Stdout), nil
}

func (c Client) WindowExists(ctx context.Context, session, name string) (bool, error) {
	windows, err := c.ListWindows(ctx, session)
	if err != nil {
		var cmdErr *execx.CommandError
		if errors.As(err, &cmdErr) {
			return false, nil
		}
		return false, err
	}
	for _, window := range windows {
		if window == name {
			return true, nil
		}
	}
	return false, nil
}

func Target(session, window string) string {
	return session + ":" + window
}

func ParsePaneList(output string) []Pane {
	var panes []Pane
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		panes = append(panes, Pane{
			ID:             parts[0],
			CurrentCommand: parts[1],
			Dead:           parts[2] == "1",
			DeadStatus:     parts[3],
		})
	}
	return panes
}

func splitLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
