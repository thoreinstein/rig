# Track rig-wlx: PR Workflow Robustness

## Overview
This track aims to improve the reliability, testability, and usability of the `rig pr` commands. It involves refactoring global flags into structs, standardizing error handling with typed errors, and adding pagination to the PR listing command.

## Functional Requirements
- **Refactor PR Flags:** Move global flags in `cmd/pr_*.go` into command-specific `Options` structs (e.g., `CreateOptions`, `ListOptions`, `ViewOptions`, `MergeOptions`).
- **Standardize Errors:** Update all `pr` subcommands to use typed errors from `pkg/errors` (specifically `GitHubError`, `WorkflowError`, and `ConfigError`) instead of generic errors.
- **PR List Pagination:** 
    - Add `--limit` flag to `rig pr list` to specify the number of PRs to fetch (default: 30).
    - Add `--page` flag to `rig pr list` to specify which page of results to retrieve.

## Non-Functional Requirements
- **Testability:** The refactoring of flags into structs should enable better unit testing of the command logic by allowing options to be passed directly to execution functions.
- **Consistency:** Error messages and types should be consistent across all PR subcommands.

## Acceptance Criteria
- [ ] `rig pr create`, `rig pr list`, `rig pr view`, and `rig pr merge` all use local `Options` structs instead of global variables for flags.
- [ ] All PR subcommands return typed errors from `pkg/errors` when failures occur (e.g., GitHub API errors are wrapped in `GitHubError`).
- [ ] `rig pr list --limit 10` returns exactly 10 PRs (if available).
- [ ] `rig pr list --page 2` returns the second page of PR results.
- [ ] Unit tests are added/updated for at least one PR command demonstrating the use of the `Options` struct for mocking/testing.

## Out of Scope
- Implementing interactive pagination (e.g., "Load More" prompts).
- Changing the underlying GitHub API client logic beyond error handling and pagination parameters.
