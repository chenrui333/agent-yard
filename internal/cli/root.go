package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

type App struct {
	configPath string
	out        io.Writer
	errOut     io.Writer
}

func Execute() error {
	return NewRootCommand(os.Stdout, os.Stderr).Execute()
}

func NewRootCommand(out, errOut io.Writer) *cobra.Command {
	app := &App{out: out, errOut: errOut}
	cmd := &cobra.Command{
		Use:           "yard",
		Short:         "Local tmux/worktree orchestration for coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&app.configPath, "config", config.DefaultFile, "path to yard.yaml")
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.AddCommand(
		app.newInitCmd(),
		app.newStatusCmd(),
		app.newBoardCmd(),
		app.newDoctorCmd(),
		app.newAttachCmd(),
		app.newCaptureCmd(),
		app.newWorktreeCmd(),
		app.newLaunchCmd(),
		app.newLaunchWaveCmd(),
		app.newWaveCmd(),
		app.newPRCmd(),
		app.newReviewLocalCmd(),
		app.newReviewPRCmd(),
		app.newSyncCmd(),
		app.newClaimCmd(),
		app.newSetStatusCmd(),
		app.newGCCmd(),
	)
	return cmd
}

func (a *App) baseDir() string {
	return config.BaseDir(a.configPath)
}

func (a *App) taskPath() string {
	return filepath.Join(a.baseDir(), task.DefaultFile)
}

func (a *App) promptDir() string {
	return filepath.Join(a.baseDir(), "prompts")
}

func (a *App) yardPath(parts ...string) string {
	items := append([]string{a.baseDir(), ".yard"}, parts...)
	return filepath.Join(items...)
}

func (a *App) loadConfig() (config.Config, error) {
	return config.Load(a.configPath)
}

func (a *App) loadState() (config.Config, task.Ledger, task.Store, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return config.Config{}, task.Ledger{}, task.Store{}, err
	}
	store := task.NewStore(a.taskPath())
	ledger, err := store.Load()
	if err != nil {
		return config.Config{}, task.Ledger{}, task.Store{}, err
	}
	return cfg, ledger, store, nil
}

func (a *App) taskWorktreePath(cfg config.Config, item task.Task) string {
	if item.Worktree != "" {
		return config.ResolvePath(a.configPath, item.Worktree)
	}
	if item.Branch == "" {
		return ""
	}
	root := config.WorktreeRootPath(a.configPath, cfg)
	return filepath.Clean(filepath.Join(root, cfg.Worktrees.Prefix+item.Branch))
}

func (a *App) promptFile(kind, id string) string {
	return a.yardPath("runs", id, kind+".md")
}

func (a *App) printf(format string, args ...any) {
	fmt.Fprintf(a.out, format, args...)
}

func taskIssue(cfg config.Config, item task.Task) int {
	if item.Issue != 0 {
		return item.Issue
	}
	return cfg.GitHub.Issue
}
