# Changelog

All notable changes to this project will be documented in this file.

## v0.0.2 - 2026-05-03

### Added

- GitHub issue checkbox import through `yard sync issue --write` with section, limit, ID prefix, and branch prefix controls.
- Hardened `yard pr` flow with local preflights, default branch push, existing PR detection, and `--no-push` / `--allow-behind` options.
- `yard ready` merge-readiness gate for local worktree state, pushed branch state, PR merge/review/check status, and paired review-lane TODO findings.
- Live tmux `impl-*` lane reservations during wave planning, preparation, and launch.
- `yard gc --prune --merged` cleanup for merged task run state and clean merged PR review worktrees.

### Changed

- `yard launch-wave` now acts as a compatibility alias for the safer `yard wave launch` path.
- Documentation now describes the production paired-workset loop and 0.0.2 release workflow.

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
