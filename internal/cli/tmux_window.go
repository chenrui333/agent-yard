package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

type tmuxWindowAction string

const (
	tmuxWindowNew     tmuxWindowAction = "new"
	tmuxWindowSend    tmuxWindowAction = "send"
	tmuxWindowRespawn tmuxWindowAction = "respawn"
	tmuxWindowReplace tmuxWindowAction = "replace"
)

type tmuxWindowPlan struct {
	session string
	window  string
	action  tmuxWindowAction
}

func addWindowReuseFlags(cmd *cobra.Command, opts *launchOptions) {
	cmd.Flags().BoolVar(&opts.reuseIdle, "reuse-idle", false, "reuse an existing tmux window only when panes are idle shells or dead")
	cmd.Flags().BoolVar(&opts.replaceWindow, "replace-window", false, "kill and recreate an existing tmux window before launching")
}

func planTmuxWindow(ctx context.Context, client tmux.Client, session, window string, opts *launchOptions) (tmuxWindowPlan, error) {
	if opts == nil {
		opts = &launchOptions{}
	}
	if opts.reuseIdle && opts.replaceWindow {
		return tmuxWindowPlan{}, fmt.Errorf("--reuse-idle and --replace-window cannot be used together")
	}
	plan := tmuxWindowPlan{session: session, window: window, action: tmuxWindowNew}
	exists, err := client.WindowExists(ctx, session, window)
	if err != nil {
		return tmuxWindowPlan{}, err
	}
	if !exists {
		return plan, nil
	}
	if opts.replaceWindow {
		plan.action = tmuxWindowReplace
		return plan, nil
	}
	if opts.reuseIdle {
		panes, err := client.ListPanes(ctx, tmux.Target(session, window))
		if err != nil {
			return tmuxWindowPlan{}, err
		}
		action, err := reusableWindowAction(panes)
		if err != nil {
			return tmuxWindowPlan{}, fmt.Errorf("tmux window %s is not idle: %w", window, err)
		}
		plan.action = action
		return plan, nil
	}
	return tmuxWindowPlan{}, fmt.Errorf("tmux window %s already exists; use --reuse-idle for an idle shell pane or --replace-window to recreate it", window)
}

func executeTmuxWindowPlan(ctx context.Context, client tmux.Client, plan tmuxWindowPlan, command string) error {
	target := tmux.Target(plan.session, plan.window)
	switch plan.action {
	case tmuxWindowNew:
		if err := client.NewWindow(ctx, plan.session, plan.window); err != nil {
			return err
		}
		return client.SendKeys(ctx, target, command)
	case tmuxWindowSend:
		return client.SendKeys(ctx, target, command)
	case tmuxWindowRespawn:
		return client.RespawnPane(ctx, target, command)
	case tmuxWindowReplace:
		if err := client.KillWindow(ctx, target); err != nil {
			return err
		}
		if err := client.NewWindow(ctx, plan.session, plan.window); err != nil {
			return err
		}
		return client.SendKeys(ctx, target, command)
	default:
		return fmt.Errorf("unknown tmux window action %q", plan.action)
	}
}

func reusableWindowAction(panes []tmux.Pane) (tmuxWindowAction, error) {
	if len(panes) == 0 {
		return "", fmt.Errorf("no panes found")
	}
	dead := 0
	idle := 0
	for _, pane := range panes {
		if pane.Dead {
			dead++
			continue
		}
		if !isIdleShell(pane.CurrentCommand) {
			return "", fmt.Errorf("pane %s is running %s", emptyAs(pane.ID, "unknown"), emptyAs(pane.CurrentCommand, "unknown"))
		}
		idle++
	}
	if idle > 0 && dead == 0 {
		return tmuxWindowSend, nil
	}
	if idle == 0 && dead > 0 {
		return tmuxWindowRespawn, nil
	}
	return "", fmt.Errorf("mixed idle and dead panes require --replace-window")
}

func isIdleShell(command string) bool {
	switch strings.TrimSpace(command) {
	case "bash", "sh", "zsh", "fish":
		return true
	default:
		return false
	}
}
