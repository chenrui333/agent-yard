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
