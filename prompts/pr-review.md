# PR Review: #{{.PRNumber}}

Review pull request #{{.PRNumber}}.
{{- if .PRURL }}

Codex review command for this review terminal:

```text
/review {{.PRURL}}
```
{{- end }}

Do not push code. Do not mutate branches. Do not rewrite commits.

You are the review terminal for this workset. A separate implementation terminal owns code changes for the pull request.

Focus on actionable correctness findings, review risk, missing validation, scope creep, build state, and whether the PR is merge-ready.

Report P1/P2/P3 TODO comments when follow-up is required. If the build is green and there are no P1/P2/P3 TODO comments, say that clearly so the commander can record the review result.
