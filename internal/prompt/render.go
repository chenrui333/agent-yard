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
	KindCommander   = "commander"
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
	Objective  string
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
	if data.PRURL == "" {
		data.PRURL = reviewURL(data.Config, data.PRNumber)
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
	KindCommander: `{{- if .Objective }}/goal {{.Objective}}{{ else }}/goal Coordinate this agent-yard session until the assigned maintenance goal is complete, explicitly paused, or handed off.{{ end }}

# Commander Workset Orchestration

You are the commander terminal for this repository's agent-yard session. Keep this session running until the goal is reached; do not exit after a single status pass.

## Role

- Own workload triage, task selection, lane assignment, and PR loop decisions.
- Use yard commands to keep tasks.yaml as the execution ledger.
- Use beads/bd for longer-lived backlog, memory, dependency, or campaign context when available.
- Keep worker and reviewer work in separate tmux terminals.
- Assign goal discovery, implementation, and guardrail audit work to worker lanes as explicit objectives.
- Do not use reviewer lanes as commander guardrails; reviewers inspect worker output only.
- Do not edit product code unless explicitly switching into a worker role.

## Operating Loop

1. Refresh the board with yard board and inspect details with yard show <task-id>.
2. Use yard wave plan, yard wave prepare, and yard wave launch to start worker lanes.
3. Start or attach reviewer lanes for PRs; reviewers have full local access for inspection, build, and tests but do not own code changes.
4. Use worker lanes, not reviewer lanes, for commander-side goal discovery or guardrail audits.
5. Route P1/P2/P3 review findings back to the assigned worker lane.
6. After meaningful commits, update PR title/body so reviewers see current scope.
7. Record a clear review outcome with yard review-result <task-id> --lane <lane> once the reviewer reports no P1/P2/P3 TODO comments.
8. Gate completion with yard ready <task-id> --review-lane <lane> --write.

## Memory

- If beads or bd is available, use it as optional persistent memory and backlog support.
- Prefer read-only beads queries from worker/reviewer terminals unless the commander explicitly assigns a memory update.
- Keep yard tasks small enough to map cleanly to one worktree and one worker/reviewer pair.
`,
	KindImplement: `# Implementation Task: {{.Task.ID}}

You are working on task {{.Task.ID}} from issue #{{.Issue}}.

- Checkbox: {{.Task.Checkbox}}
- Branch: {{.Task.Branch}}
- Worktree: {{.Task.Worktree}}
- Base branch: {{.BaseBranch}}
- Remote: {{.Remote}}

## Guardrails

- Stay inside the assigned worktree.
- Do not touch unrelated services or files.
- Do not force-push unless explicitly requested.
- Use signed-off commits if configured.
- Prefer focused diffs.
- Update docs and tests when relevant.
- Keep pull requests scoped to this task and reference the issue with Refs #{{.Issue}}.

## Paired Workset Loop

- You are the implementation terminal for this workset.
- A separate review terminal may inspect the same worktree or pull request.
- Treat P1/P2/P3 review findings and TODO comments as required follow-up work.
- Make focused follow-up commits in this worktree when review feedback is valid.
- Do not take over the review terminal's role; report what changed and any PR title/body updates the commander should make.

## Project-Specific Correctness

- Read the task text and linked issue carefully before changing code.
- Verify the upstream or framework contract before wiring new behavior.
- Use canonical identifiers and state formats expected by the target project.
- Avoid generated name or label collisions; keep names stable and unique.
- Add focused tests for filters, selectors, parsing, or discovery behavior when relevant.
- Document unsupported, deleted, non-refreshable, or intentionally skipped cases instead of forcing partial support.
- Keep shared registration, docs, and helper changes minimal so parallel task lanes do not conflict.

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

You are the review terminal for this workset. A separate implementation terminal owns code changes in the assigned worktree.

You have full local access for inspection, build, and tests, but you do not own implementation changes. Do not commit, push, or rewrite files. Focus on correctness, test gaps, and scope control.

Report findings first, ordered by severity, with file and line references where possible. Use P1/P2/P3 severity for actionable TODO comments. If there are no P1/P2/P3 TODO comments, say that clearly and call out any residual test gaps.
`,
	KindPRReview: `# PR Review: #{{.PRNumber}}

Review pull request #{{.PRNumber}}.
{{- if .PRURL }}

Codex review command for this review terminal:

` + "```text\n/review {{.PRURL}}\n```\n" + `{{- end }}

Do not push code. Do not mutate branches. Do not rewrite commits.

You are the review terminal for this workset. A separate implementation terminal owns code changes for the pull request.

Focus on actionable correctness findings, review risk, missing validation, scope creep, build state, and whether the PR is merge-ready.

Report P1/P2/P3 TODO comments when follow-up is required. If the build is green and there are no P1/P2/P3 TODO comments, say that clearly so the commander can record the review result.
`,
}

func DefaultTemplate(kind string) (string, bool) {
	source, ok := embeddedDefaults[kind]
	return source, ok
}

func Kinds() []string {
	return []string{KindCommander, KindImplement, KindLocalReview, KindPRReview}
}

func reviewURL(cfg config.Config, prNumber int) string {
	if cfg.GitHub.Owner == "" || cfg.GitHub.Repo == "" || prNumber == 0 {
		return ""
	}
	return fmt.Sprintf("https://%s/%s/%s/pull/%d", config.GitHubHost(cfg), cfg.GitHub.Owner, cfg.GitHub.Repo, prNumber)
}
