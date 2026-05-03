# Implementation Task: {{.Task.ID}}

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

## Validation Examples

- GOWORK=off go test ./providers/aws -run 'Test<ServiceOrFeature>' -count=1
- GOWORK=off go test ./providers/aws -count=1
- git diff --check
- golangci-lint run --new-from-rev="$(git merge-base HEAD origin/main)" ./providers/aws --output.text.path stdout --allow-parallel-runners
