package cli

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/ghx"
	issuex "github.com/chenrui333/agent-yard/internal/issue"
	"github.com/chenrui333/agent-yard/internal/task"
	"github.com/spf13/cobra"
)

func (a *App) newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync external control-plane state into the local ledger",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.newSyncIssueCmd())
	return cmd
}

func (a *App) newSyncIssueCmd() *cobra.Command {
	var write bool
	var limit int
	var section string
	var idPrefix string
	var branchPrefix string
	cmd := &cobra.Command{
		Use:   "issue <issue-number>",
		Short: "Import GitHub issue checkboxes into tasks.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue number %q", args[0])
			}
			if issueNumber <= 0 {
				return fmt.Errorf("invalid issue number %q: must be > 0", args[0])
			}
			if limit < 0 {
				return fmt.Errorf("invalid --limit %d: must be >= 0", limit)
			}
			cfg, err := a.loadConfig()
			if err != nil {
				return err
			}
			if err := ghx.EnsureExists(); err != nil {
				return err
			}
			issue, err := ghx.New().IssueView(cmd.Context(), config.RepoPath(a.configPath, cfg), config.GitHubRepoArg(cfg), issueNumber)
			if err != nil {
				return err
			}
			return a.runSyncIssue(issueNumber, issue.Title, issue.URL, issue.Body, issuex.ImportOptions{
				IssueNumber:  issueNumber,
				Limit:        limit,
				Section:      section,
				IDPrefix:     idPrefix,
				BranchPrefix: branchPrefix,
			}, write)
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "write imported tasks to tasks.yaml")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum new tasks to import; 0 means no limit")
	cmd.Flags().StringVar(&section, "section", "", "only import checkboxes under the matching Markdown heading slug")
	cmd.Flags().StringVar(&idPrefix, "id-prefix", "", "task ID prefix; defaults to issue-<number>-")
	cmd.Flags().StringVar(&branchPrefix, "branch-prefix", "", "branch prefix; defaults to the generated task ID")
	return cmd
}

func (a *App) runSyncIssue(issueNumber int, title, url, body string, opts issuex.ImportOptions, write bool) error {
	if issueNumber <= 0 {
		return fmt.Errorf("invalid issue number %d: must be > 0", issueNumber)
	}
	if opts.Limit < 0 {
		return fmt.Errorf("invalid --limit %d: must be >= 0", opts.Limit)
	}
	store := task.NewStore(a.taskPath())
	boxes := issuex.ParseCheckboxes(body)
	if !write {
		ledger, err := store.Load()
		if err != nil {
			return err
		}
		result := issuex.ImportTasks(ledger, boxes, opts)
		a.printf("issue #%d: %s\n%s\n", issueNumber, title, url)
		a.printf("would add %d task(s), skipped %d checkbox(es)\n", result.Added, result.Skipped)
		return a.renderIssueImport(result.Tasks)
	}

	var result issuex.ImportResult
	if err := store.WithLock(func(ledger *task.Ledger) error {
		result = issuex.ImportTasks(*ledger, boxes, opts)
		ledger.Tasks = append(ledger.Tasks, result.Tasks...)
		return task.Validate(*ledger)
	}); err != nil {
		return err
	}
	a.printf("issue #%d: %s\n%s\n", issueNumber, title, url)
	a.printf("added %d task(s), skipped %d checkbox(es)\n", result.Added, result.Skipped)
	return a.renderIssueImport(result.Tasks)
}

func (a *App) renderIssueImport(tasks []task.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TASK\tBRANCH\tSERVICE_FAMILY\tCHECKBOX"); err != nil {
		return err
	}
	for _, item := range tasks {
		checkbox := strings.ReplaceAll(item.Checkbox, "\t", " ")
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.ID, item.Branch, item.ServiceFamily, checkbox); err != nil {
			return err
		}
	}
	return tw.Flush()
}
