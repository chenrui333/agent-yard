# Agent Yard Implementation Plan

## Current State

agent-yard is a Go CLI named `yard` for generic tmux-backed coding-agent orchestration. It uses git worktrees for task isolation, `tasks.yaml` as the local ledger, and optional GitHub issues and pull requests as collaboration boundaries.

## Package Layout

- `cmd/yard/main.go`: binary entrypoint.
- `internal/cli`: Cobra command definitions and workflow orchestration.
- `internal/config`: `yard.yaml` schema, defaults, path resolution, load/save.
- `internal/task`: `tasks.yaml` schema, status constants, validation, lookup, locked atomic store writes.
- `internal/issue`: Markdown checkbox parsing and task import planning.
- `internal/execx`, `internal/gitx`, `internal/ghx`, `internal/tmux`: thin wrappers over system CLIs.
- `internal/agent`: shell quoting, tmux launch command construction, lane/window naming helpers.
- `internal/prompt`: prompt rendering from files with embedded fallbacks.
- `internal/status` and `internal/wave`: coordinator views and wave selection logic.

## Production Readiness Scope

- Keep long-running agent processes inside tmux windows.
- Use git worktrees as implementation isolation boundaries.
- Import GitHub issue checkboxes into stable task entries while preserving existing ledger state.
- Use wave plan/prepare/launch for larger worksets, reserving lanes already used by active ledger tasks or live `impl-*` tmux windows.
- Harden PR creation with local preflights, default branch push, and existing-PR detection.
- Use `yard ready` as the final workset gate before marking a task `merge_ready`.
- Keep cleanup report-only by default; require explicit prune flags for removal.

## Out of Scope

- No web dashboard, daemon, terminal multiplexer, TUI, SQLite database, MCP integration, or Ghostty/iTerm automation.
- No autonomous supervisor or automatic `/review` invocation; the dispatcher owns the paired implementation/review terminal loop.
- No campaign-specific built-in rules. Terraformer AWS is a good example campaign, but project-specific rules belong in local prompt templates.

## Test Plan

- Unit test config, task ledger, issue parsing/import, prompt rendering, git/gh/tmux parsing, status rendering, readiness helpers, and wave selection.
- Integration test init, status/board, issue sync, worktree creation, wave prepare/launch, PR creation, PR review, readiness, and cleanup flows.
- Run `go test ./...`, `go vet ./...`, GoReleaser check/snapshot, Renovate config validation, actionlint when available, and `git diff --check` before release handoff.
