package cli

import (
	"fmt"

	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

func (a *App) newSetStatusCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "set-status <task-id> <status>",
		Short: "Set a task status in tasks.yaml",
		Long:  "Set a task status in tasks.yaml. Valid statuses: " + validStatusValues(),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSetStatus(args[0], args[1], note)
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "optional status note to store with the task")
	return cmd
}

func (a *App) runSetStatus(taskID, statusValue, note string) error {
	status, err := task.ParseStatus(statusValue)
	if err != nil {
		return err
	}
	store := task.NewStore(a.taskPath())
	if err := store.Update(taskID, func(item *task.Task) error {
		item.Status = status
		if note != "" {
			item.Note = note
		}
		return nil
	}); err != nil {
		return err
	}
	if note != "" {
		a.printf("%s -> %s (%s)\n", taskID, status, note)
		return nil
	}
	a.printf("%s -> %s\n", taskID, status)
	return nil
}

func validStatusValues() string {
	values := task.StatusList()
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return fmt.Sprint(out)
}
