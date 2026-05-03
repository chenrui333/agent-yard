# Agent Yard MVP Implementation Plan

## Current Repo State

- The repository is empty except for Git metadata and the initial directories created for this implementation.
- There is no existing Go module, command entrypoint, package layout, README, or test suite.
- The remote repository exists, but there are no commits or default branch metadata yet.

## Intended Package Layout

- `cmd/yard/main.go`: binary entrypoint.
- `internal/cli`: Cobra command definitions and flag parsing.
- `internal/config`: `yard.yaml` schema, defaults, path resolution, load/save.
- `internal/task`: `tasks.yaml` schema, status constants, validation, lookup, locked atomic store writes.
- `internal/execx`: structured wrapper around `os/exec`.
- `internal/gitx`, `internal/ghx`, `internal/tmux`: thin wrappers over system CLIs.
- `internal/agent`: shell quoting, tmux launch command construction, lane/window naming helpers.
- `internal/prompt`: prompt rendering from files with embedded fallbacks.
- `internal/status`: textual status and board rendering.
- `prompts/`: editable default prompt templates.

## MVP Scope

- Build a Cobra CLI named `yard` with the requested command surface.
- Keep long-running agent processes inside tmux windows.
- Use git worktrees as implementation isolation boundaries.
- Use `tasks.yaml` as the editable task ledger while deriving status from git, tmux, and gh where cheap.
- Support `--dry-run` for commands that would launch agents or mutate GitHub.
- Keep placeholders non-crashing and explicit where full workflow automation is deferred.

## Deferred Scope

- No web dashboard, daemon, terminal multiplexer, TUI, SQLite database, MCP integration, or Ghostty/iTerm automation.
- No autonomous supervisor or retry loop.
- No full GitHub issue checkbox reconciliation beyond an MVP issue-view/sync placeholder.
- No full temporary PR-review worktree lifecycle unless it proves safe in a later pass.
- No Homebrew formula yet, only package-friendly project shape.

## Risks

- Shell quoting must be centralized so tmux receives a safe command string.
- `yard status` should tolerate missing external tools and stale ledger values without crashing.
- Worktree creation must refuse conflicting dirty paths instead of overwriting local work.
- Concurrent ledger writes must be locked and atomic.
- Generated docs and examples should avoid machine-local absolute paths.

## Test Plan

- Unit test config defaults, load/save behavior, and path expansion.
- Unit test task ledger validation, lookup/update, and locked atomic saves.
- Unit test prompt rendering with embedded defaults and template files.
- Unit test shell quoting and tmux launch command construction.
- Unit test git worktree porcelain parsing.
- Unit test minimal gh PR JSON parsing.
- Run `gofmt ./...`, `go test ./...`, and `go vet ./...` before handoff.
