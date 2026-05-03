# Changelog

All notable changes to this project will be documented in this file.

## v0.0.1 - 2026-05-03

### Added

- Initial `yard` CLI built with Cobra.
- Local YAML config and task ledger support through `yard.yaml` and `tasks.yaml`.
- Git worktree creation for task-isolated implementation lanes.
- tmux-backed launch flows for implementation, local review, and PR review lanes.
- GitHub CLI helpers for issue sync placeholders, task claims, and PR dry-runs.
- Text status and board views that derive worktree, dirty, tmux, and PR state where possible.
- Prompt templates with embedded defaults for implementation and review agents.
- GoReleaser release automation for macOS/Linux x86_64 and arm64 archives.
- GitHub Actions CI/release workflows and Renovate automerge configuration.
