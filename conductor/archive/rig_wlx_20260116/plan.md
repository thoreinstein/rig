# Implementation Plan - Track rig-wlx: PR Workflow Robustness

## Phase 1: PR Flags Refactoring (Testability) [checkpoint: 2d0f499]
Refactor PR commands to move global flags into command-specific structs to improve testability.

- [x] Task: Create `CreateOptions` struct and refactor `pr create` command logic.
- [x] Task: Create `ListOptions` struct and refactor `pr list` command logic.
- [x] Task: Create `ViewOptions` struct and refactor `pr view` command logic.
    - [x] Define `ViewOptions` struct in `cmd/pr_view.go`.
    - [x] Update `init()` to bind flags to the struct.
    - [x] Refactor execution logic into a function that accepts `ViewOptions`.
    - [x] Write/update tests in `cmd/pr_view_test.go`.
- [x] Task: Create `MergeOptions` struct and refactor `pr merge` command logic.
    - [x] Define `MergeOptions` struct in `cmd/pr_merge.go`.
    - [x] Update `init()` to bind flags to the struct.
    - [x] Refactor execution logic into a function that accepts `MergeOptions`.
    - [x] Write/update tests in `cmd/pr_merge_test.go`.
- [x] Task: Conductor - User Manual Verification 'Phase 1: PR Flags Refactoring' (Protocol in workflow.md)

## Phase 2: Typed Errors Standardization [checkpoint: 24f7227]
Standardize error handling across PR commands using `pkg/errors`.

- [x] Task: Update `pr create` to use typed errors.
    - [x] Wrap GitHub API errors with `errors.NewGitHubErrorWithCause`.
    - [x] Wrap workflow-related errors with `errors.NewWorkflowErrorWithCause`.
    - [x] Verify error handling with tests.
- [x] Task: Update `pr list` to use typed errors.
    - [x] Ensure all error paths return typed errors.
    - [x] Verify error handling with tests.
- [x] Task: Update `pr view` to use typed errors.
    - [x] Ensure all error paths return typed errors.
    - [x] Verify error handling with tests.
- [x] Task: Update `pr merge` to use typed errors.
    - [x] Ensure all error paths return typed errors.
    - [x] Verify error handling with tests.
- [x] Task: Conductor - User Manual Verification 'Phase 2: Typed Errors Standardization' (Protocol in workflow.md)

## Phase 3: PR List Pagination [checkpoint: 80acff4]
Implement pagination support for the `pr list` command.

- [x] Task: Update `ListOptions` to include `Limit` and `Page`.
    - [x] Add `Limit` and `Page` fields to `ListOptions`.
    - [x] Bind `--limit` and `--page` flags in `cmd/pr_list.go`.
- [x] Task: Implement pagination logic in PR listing.
    - [x] Update the GitHub client call in `cmd/pr_list.go` to pass pagination parameters.
    - [x] Ensure output reflects the requested subset of PRs.
- [x] Task: Verify pagination with unit and integration tests.
    - [x] Write tests for various limit/page combinations.
    - [x] Verify that the default behavior (limit 30, page 1) remains unchanged.
- [x] Task: Conductor - User Manual Verification 'Phase 3: PR List Pagination' (Protocol in workflow.md)
