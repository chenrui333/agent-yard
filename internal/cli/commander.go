package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chenrui333/agent-yard/internal/agent"
	"github.com/chenrui333/agent-yard/internal/execx"
	"github.com/chenrui333/agent-yard/internal/prompt"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

func (a *App) newCommanderCmd() *cobra.Command {
	opts := &launchOptions{}
	goal := ""
	cmd := &cobra.Command{
		Use:   "commander",
		Short: "Launch the commander orchestration terminal",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runCommander(cmd, opts, goal)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the commander command without launching")
	cmd.Flags().BoolVar(&opts.force, "force", false, "reuse an existing commander window")
	cmd.Flags().StringVar(&goal, "goal", "", "specific commander goal for the Codex /goal line")
	return cmd
}

func (a *App) runCommander(cmd *cobra.Command, opts *launchOptions, goal string) error {
	cfg, err := a.loadConfig()
	if err != nil {
		return err
	}
	workdir, err := filepath.Abs(a.baseDir())
	if err != nil {
		return err
	}
	promptPath, err := filepath.Abs(a.promptFile(prompt.KindCommander, "commander"))
	if err != nil {
		return err
	}
	renderer := prompt.Renderer{Dir: a.promptDir()}
	data := prompt.Data{Config: cfg, Objective: strings.TrimSpace(goal)}
	if opts.dryRun {
		if _, err := renderer.Render(prompt.KindCommander, data); err != nil {
			return err
		}
	} else if err := renderer.RenderToFile(prompt.KindCommander, data, promptPath); err != nil {
		return err
	}
	window := agent.SanitizeWindowName("commander")
	launchCommand := agent.BuildLaunchCommand(workdir, promptPath, cfg.Agents.Commander)
	if opts.dryRun {
		a.printf("window: %s\ncommand: %s\n", window, launchCommand)
		return nil
	}
	if err := tmux.EnsureExists(); err != nil {
		return err
	}
	if _, err := execx.LookPath(cfg.Agents.Commander.Command); err != nil {
		return err
	}
	tmuxClient := tmux.New()
	ctx := cmd.Context()
	if err := tmuxClient.EnsureSession(ctx, cfg.Session); err != nil {
		return err
	}
	exists, err := tmuxClient.WindowExists(ctx, cfg.Session, window)
	if err != nil {
		return err
	}
	if exists && !opts.force {
		return fmt.Errorf("tmux window %s already exists", window)
	}
	if !exists {
		if err := tmuxClient.NewWindow(ctx, cfg.Session, window); err != nil {
			return err
		}
	}
	return tmuxClient.SendKeys(ctx, tmux.Target(cfg.Session, window), launchCommand)
}
