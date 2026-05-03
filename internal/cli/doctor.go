package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/execx"
	"github.com/chenrui333/agent-yard/internal/gitx"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/chenrui333/agent-yard/internal/tmux"
	"github.com/spf13/cobra"
)

type doctorRow struct {
	Name   string
	Status string
	Detail string
}

func (a *App) newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local dependencies and yard configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runDoctor(cmd.Context())
		},
	}
}

func (a *App) runDoctor(ctx context.Context) error {
	var rows []doctorRow
	failures := 0
	add := func(name string, err error, detail string) {
		status := "ok"
		if err != nil {
			status = "fail"
			detail = err.Error()
			failures++
		}
		rows = append(rows, doctorRow{Name: name, Status: status, Detail: detail})
	}
	warn := func(name, detail string) {
		rows = append(rows, doctorRow{Name: name, Status: "warn", Detail: detail})
	}

	cfg, err := a.loadConfig()
	add("config", err, a.configPath)
	if err != nil {
		if renderErr := a.renderDoctorRows(rows); renderErr != nil {
			return renderErr
		}
		return fmt.Errorf("doctor found %d failure(s)", failures)
	}

	add("git", gitx.EnsureExists(), "system git")
	githubEnabled := githubConfigured(cfg)
	if err := execxLook("gh"); err != nil {
		if githubEnabled {
			add("gh", err, "GitHub CLI")
		} else {
			warn("gh", "GitHub CLI missing; required for GitHub commands")
		}
	} else {
		add("gh", nil, "GitHub CLI")
	}
	add("tmux", tmux.EnsureExists(), "tmux backend")
	add("agent implementation", execxLook(cfg.Agents.Implementation.Command), cfg.Agents.Implementation.Command)
	add("agent local_review", execxLook(cfg.Agents.LocalReview.Command), cfg.Agents.LocalReview.Command)
	add("agent pr_review", execxLook(cfg.Agents.PRReview.Command), cfg.Agents.PRReview.Command)

	repo := config.RepoPath(a.configPath, cfg)
	add("repo path", dirExists(repo), repo)
	root := config.WorktreeRootPath(a.configPath, cfg)
	add("worktree root", writableDir(root), root)

	store := task.NewStore(a.taskPath())
	if _, err := store.Load(); err != nil {
		add("tasks.yaml", err, a.taskPath())
	} else {
		add("tasks.yaml", nil, a.taskPath())
	}

	if execx.Exists("gh") {
		_, err := execx.Runner{}.Run(ctx, execx.Command{Name: "gh", Args: []string{"auth", "status", "--hostname", githubHost(cfg)}})
		if err != nil && !githubEnabled {
			warn("gh auth", "not authenticated; required for GitHub commands")
		} else {
			add("gh auth", err, "authenticated GitHub CLI for "+githubHost(cfg))
		}
	}
	if execx.Exists("git") && dirExists(repo) == nil {
		baseRef := cfg.DefaultRemote + "/" + cfg.BaseBranch
		add("base ref", gitx.New().VerifyRef(ctx, repo, baseRef), baseRef)
	}
	if execx.Exists("tmux") {
		exists, err := tmux.New().HasSession(ctx, cfg.Session)
		if err != nil {
			add("tmux session", err, cfg.Session)
		} else if exists {
			add("tmux session", nil, cfg.Session)
		} else {
			warn("tmux session", cfg.Session+" missing; launch will create it")
		}
	}

	if err := a.renderDoctorRows(rows); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("doctor found %d failure(s)", failures)
	}
	return nil
}

func githubConfigured(cfg config.Config) bool {
	return cfg.GitHub.Host != "" || cfg.GitHub.Owner != "" || cfg.GitHub.Repo != "" || cfg.GitHub.Issue != 0
}

func githubHost(cfg config.Config) string {
	if cfg.GitHub.Host != "" {
		return cfg.GitHub.Host
	}
	return "github.com"
}

func (a *App) renderDoctorRows(rows []doctorRow) error {
	tw := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "CHECK\tSTATUS\tDETAIL"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", row.Name, row.Status, row.Detail); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func execxLook(name string) error {
	if name == "" {
		return fmt.Errorf("command is empty")
	}
	_, err := execx.LookPath(name)
	return err
}

func dirExists(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func writableDir(path string) error {
	stat, err := os.Stat(path)
	if err == nil {
		if !stat.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		return writableExistingDir(path)
	}
	if !os.IsNotExist(err) {
		return err
	}
	return writableParentDir(path)
}

func writableParentDir(path string) error {
	parent := filepath.Dir(filepath.Clean(path))
	for {
		stat, err := os.Stat(parent)
		if err == nil {
			if !stat.IsDir() {
				return fmt.Errorf("%s is not a directory", parent)
			}
			return writableExistingDir(parent)
		}
		if !os.IsNotExist(err) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return err
		}
		parent = next
	}
}

func writableExistingDir(path string) error {
	tmp, err := os.CreateTemp(path, ".yard-doctor-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(filepath.Clean(name))
}
