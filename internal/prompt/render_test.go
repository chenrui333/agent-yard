package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/task"
)

func TestRenderEmbeddedDefault(t *testing.T) {
	rendered, err := (Renderer{}).Render(KindImplement, Data{
		Config: config.Default(),
		Task: task.Task{
			ID:       "aws-route53",
			Issue:    338,
			Checkbox: "Route53 resources",
			Branch:   "aws-route53-resources",
			Worktree: "$HOME/src/terraformer.aws-route53-resources",
		},
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(rendered, "aws-route53") || !strings.Contains(rendered, "Route53 resources") {
		t.Fatalf("rendered prompt missing task fields:\n%s", rendered)
	}
	if !strings.Contains(rendered, "implementation terminal for this workset") {
		t.Fatalf("rendered prompt missing paired workset guidance:\n%s", rendered)
	}
}

func TestRenderCommanderIncludesGoal(t *testing.T) {
	rendered, err := (Renderer{}).Render(KindCommander, Data{Objective: "finish the maintenance wave"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(rendered, "/goal finish the maintenance wave") {
		t.Fatalf("rendered commander prompt missing goal:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Keep this session running until the goal is reached") {
		t.Fatalf("rendered commander prompt missing long-running guidance:\n%s", rendered)
	}
	if !strings.Contains(rendered, "yard review-result <task-id> --lane <lane>") {
		t.Fatalf("rendered commander prompt missing review-result command shape:\n%s", rendered)
	}
}

func TestRenderTemplateFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "implement.md"), []byte("task={{.Task.ID}}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	rendered, err := (Renderer{Dir: dir}).Render(KindImplement, Data{Task: task.Task{ID: "one"}})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if rendered != "task=one" {
		t.Fatalf("rendered = %q; want task=one", rendered)
	}
}

func TestRenderPRReviewIncludesReviewLoop(t *testing.T) {
	cfg := config.Default()
	cfg.GitHub.Owner = "owner"
	cfg.GitHub.Repo = "repo"
	rendered, err := (Renderer{}).Render(KindPRReview, Data{
		Config:   cfg,
		PRNumber: 123,
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	for _, want := range []string{
		"```text",
		"/review https://github.com/owner/repo/pull/123",
		"```",
		"review terminal for this workset",
		"no P1/P2/P3 TODO comments",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered prompt missing %q:\n%s", want, rendered)
		}
	}
}

func TestRenderPRReviewUsesConfiguredGitHubHost(t *testing.T) {
	cfg := config.Default()
	cfg.GitHub.Host = "https://ghe.example.com/"
	cfg.GitHub.Owner = "owner"
	cfg.GitHub.Repo = "repo"
	rendered, err := (Renderer{}).Render(KindPRReview, Data{
		Config:   cfg,
		PRNumber: 123,
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(rendered, "/review https://ghe.example.com/owner/repo/pull/123") {
		t.Fatalf("rendered prompt missing enterprise review URL:\n%s", rendered)
	}
	if strings.Contains(rendered, "https://github.com/owner/repo/pull/123") {
		t.Fatalf("rendered prompt used github.com instead of configured host:\n%s", rendered)
	}
}
