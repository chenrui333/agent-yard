package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/prompt"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

func (a *App) newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create yard.yaml, tasks.yaml, prompts, and .yard directories if missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runInit()
		},
	}
}

func (a *App) runInit() error {
	if err := os.MkdirAll(a.yardPath("runs"), 0o755); err != nil {
		return fmt.Errorf("create .yard/runs: %w", err)
	}
	if err := os.MkdirAll(a.yardPath("reviews"), 0o755); err != nil {
		return fmt.Errorf("create .yard/reviews: %w", err)
	}
	if err := os.MkdirAll(a.yardPath("review-results"), 0o755); err != nil {
		return fmt.Errorf("create .yard/review-results: %w", err)
	}
	if err := os.MkdirAll(a.promptDir(), 0o755); err != nil {
		return fmt.Errorf("create prompts dir: %w", err)
	}
	if !config.Exists(a.configPath) {
		if err := config.Save(a.configPath, config.Default()); err != nil {
			return err
		}
		a.printf("created %s\n", a.configPath)
	} else {
		a.printf("exists %s\n", a.configPath)
	}
	store := task.NewStore(a.taskPath())
	if _, err := os.Stat(a.taskPath()); os.IsNotExist(err) {
		if err := store.Save(task.EmptyLedger()); err != nil {
			return err
		}
		a.printf("created %s\n", a.taskPath())
	} else if err != nil {
		return fmt.Errorf("stat tasks file: %w", err)
	} else {
		a.printf("exists %s\n", a.taskPath())
	}
	for _, kind := range prompt.Kinds() {
		source, _ := prompt.DefaultTemplate(kind)
		path := filepath.Join(a.promptDir(), kind+".md")
		if err := writeIfMissing(path, source); err != nil {
			return err
		}
	}
	return nil
}

func writeIfMissing(path, body string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
