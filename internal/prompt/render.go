package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/task"
)

const (
	KindImplement   = "implement"
	KindLocalReview = "local-review"
	KindPRReview    = "pr-review"
)

type Data struct {
	Task       task.Task
	Config     config.Config
	Issue      int
	PRNumber   int
	PRURL      string
	BaseBranch string
	Remote     string
}

type Renderer struct {
	Dir string
}

func (r Renderer) Render(kind string, data Data) (string, error) {
	source, err := r.templateSource(kind)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(kind).Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse %s prompt: %w", kind, err)
	}
	if data.BaseBranch == "" {
		data.BaseBranch = data.Config.BaseBranch
	}
	if data.Remote == "" {
		data.Remote = data.Config.DefaultRemote
	}
	if data.Issue == 0 {
		data.Issue = data.Task.Issue
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render %s prompt: %w", kind, err)
	}
	return out.String(), nil
}

func (r Renderer) RenderToFile(kind string, data Data, path string) error {
	rendered, err := r.Render(kind, data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create prompt dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("write prompt %s: %w", path, err)
	}
	return nil
}

func (r Renderer) templateSource(kind string) (string, error) {
	if r.Dir != "" {
		path := filepath.Join(r.Dir, kind+".md")
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("read prompt template %s: %w", path, err)
		}
	}
	source, ok := embeddedDefaults[kind]
	if !ok {
		return "", fmt.Errorf("unknown prompt kind %q", kind)
	}
	return source, nil
}

var embeddedDefaults = map[string]string{
	KindImplement: `# Implementation Task: {{.Task.ID}}

You are working on task {{.Task.ID}} from issue #{{.Issue}}.

- Checkbox: {{.Task.Checkbox}}
- Branch: {{.Task.Branch}}
- Worktree: {{.Task.Worktree}}
- Base branch: {{.BaseBranch}}

## Guardrails

- Stay inside the assigned worktree.
- Do not touch unrelated services or files.
- Do not force-push unless explicitly requested.
- Use signed-off commits if configured.
- Prefer focused diffs.
- Update docs and tests when relevant.

## Validation Examples

- GOWORK=off go test ./providers/aws -run 'Test<ServiceOrFeature>' -count=1
- GOWORK=off go test ./providers/aws -count=1
- git diff --check
- golangci-lint run --new-from-rev="$(git merge-base HEAD origin/main)" ./providers/aws --output.text.path stdout --allow-parallel-runners
`,
	KindLocalReview: `# Local Review: {{.Task.ID}}

Review the assigned worktree for task {{.Task.ID}}.

- Worktree: {{.Task.Worktree}}
- Branch: {{.Task.Branch}}
- Issue: #{{.Issue}}

Stay read-only. Do not commit, push, or rewrite files. Focus on correctness, test gaps, and scope control.
`,
	KindPRReview: `# PR Review: #{{.PRNumber}}

Review pull request #{{.PRNumber}}.

Do not push code. Do not mutate branches. Focus on actionable correctness findings, review risk, and missing validation.
`,
}

func DefaultTemplate(kind string) (string, bool) {
	source, ok := embeddedDefaults[kind]
	return source, ok
}

func Kinds() []string {
	return []string{KindImplement, KindLocalReview, KindPRReview}
}
