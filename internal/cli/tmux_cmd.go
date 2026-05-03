package cli

import (
	"fmt"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

func (a *App) newAttachCmd() *cobra.Command {
	var session string
	cmd := &cobra.Command{
		Use:   "attach [task-id]",
		Short: "Attach to the configured tmux session or a task window",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAttach(cmd, session, args)
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "tmux session override")
	return cmd
}

func (a *App) newCaptureCmd() *cobra.Command {
	var session string
	cmd := &cobra.Command{
		Use:   "capture <task-id>",
		Short: "Print the current tmux pane contents for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runCapture(cmd, session, args[0])
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "tmux session override")
	return cmd
}

func (a *App) runAttach(cmd *cobra.Command, session string, args []string) error {
	cfg, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	if session == "" {
		session = cfg.Session
	}
	target := session
	if len(args) == 1 {
		item, _, ok := ledger.Find(args[0])
		if !ok {
			return fmt.Errorf("task %q not found", args[0])
		}
		target = tmux.Target(session, agent.TaskWindowName(*item))
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	return tmux.Attach(cmd.Context(), target)
}

func (a *App) runCapture(cmd *cobra.Command, session, taskID string) error {
	cfg, ledger, _, err := a.loadState()
	if err != nil {
		return err
	}
	if session == "" {
		session = cfg.Session
	}
	item, _, ok := ledger.Find(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	target := tmux.Target(session, agent.TaskWindowName(*item))
	out, err := tmux.New().CapturePane(cmd.Context(), target)
	if err != nil {
		return err
	}
	a.printf("%s", out)
	return nil
}
