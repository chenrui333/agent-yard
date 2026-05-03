# agent-yard Architecture

agent-yard is a local orchestration CLI for coordinating multiple coding agents through tmux-backed terminals and git worktrees. It stays deliberately small: tmux owns long-running processes, git owns source isolation, GitHub is an optional collaboration boundary, and `tasks.yaml` is the local execution ledger.

## Core Model

The standard unit is a paired workset:

- one worker terminal for an assigned objective in one worktree or coordination scope
- one reviewer terminal for review of worker output in that same worktree or pull request
- one commander terminal that coordinates many worksets without taking over their terminals

Scale comes from adding more independent worksets, not from sharing worktrees or injecting multiple agents into the same terminal. Each workset should be able to progress, block, review, and finish without interfering with another workset.

## Roles

### Commander

The commander is the mother/orchestrator terminal. It runs `yard board`, `yard show`, `yard wave plan`, `yard wave prepare`, `yard wave launch`, `yard review-result`, `yard ready`, and PR metadata updates. It owns workload triage, lane assignment, follow-up routing, and final readiness gates.

The commander should not become a hidden daemon or autonomous supervisor. It is an explicit long-running terminal session with visible commands and reviewable state transitions. It should keep running until the assigned goal is reached, paused, or handed off.

For Codex, the commander prompt starts with `/goal`; pass a concrete stop condition with `yard commander --goal "..."`.

### Worker

The worker owns one assigned objective. Most workers implement code in one task/worktree, but the commander can also assign worker objectives such as goal discovery or guardrail audits. Those are still workers, not separate role kinds.

Workers have full local access for inspection, build, tests, and implementation. They should not reuse another workset's terminal or worktree. Implementation changes stay inside the assigned worktree unless the commander deliberately reassigns scope.

### Reviewer

The reviewer reviews worker output. It has full local access for inspection, build, and tests, but it does not own implementation changes or commander guardrails. It reports actionable P1/P2/P3 findings and says clearly when there are no P1/P2/P3 TODO comments.

When the reviewer is clear, the commander can record that outcome with `yard review-result` so readiness does not depend only on tmux scrollback.

## State Boundaries

- `tasks.yaml` is the execution ledger for task IDs, branches, worktrees, statuses, lanes, notes, and PR links.
- `.yard/runs/` stores generated prompts for launched terminals.
- `.yard/reviews/` stores isolated PR review worktrees.
- `.yard/review-results/` stores structured review outcomes used by readiness checks.
- tmux stores live terminal state and long-running agent processes.
- Git worktrees store code changes and local git state.

## Optional Memory With Beads

beads/bd can be used as a longer-lived memory and backlog layer when available. It is not required for yard to run.

Recommended split:

- use beads for campaign memory, dependency graphs, backlog notes, stale context, and cross-session recall
- use `tasks.yaml` for the concrete execution set currently assigned to tmux lanes
- let the commander consult and update beads
- keep worker and reviewer beads usage read-only unless the commander explicitly assigns a memory update

This keeps yard generic while still allowing richer memory support in environments that already use beads.

## Readiness Loop

1. Commander imports or curates work into `tasks.yaml`.
2. Commander launches worker lanes with wave commands.
3. Workers complete assigned objectives: implementation, goal discovery, or guardrail audit.
4. Implementation workers push focused task branches.
5. Commander opens or updates PRs.
6. Reviewers inspect worker output in separate terminals.
7. Commander routes review findings back to workers.
8. Reviewer reports no P1/P2/P3 TODO comments.
9. Commander records a structured review result and runs `yard ready <task-id> --review-lane <lane> --write`.

The loop stops only when local state, PR state, CI/review state, and reviewer output all agree that the task is merge-ready.
