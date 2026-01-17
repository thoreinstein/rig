# Implementation Plan - Refine History Analysis

## Phase 1: Query Engine Enhancements [checkpoint: be780d7]
- [x] Task: Define new query parameters in `pkg/history/types.go` 300445c
    - [x] Create `QueryOptions` struct with `ExitCode`, `MinDuration`, `SessionID`.
    - [x] Update `Search` interface signature to accept `QueryOptions`.
- [x] Task: Implement SQL query builder updates in `pkg/history/sqlite.go` baae811
    - [x] Write tests for `BuildQuery` ensuring correct WHERE clauses for new filters.
    - [x] Implement `BuildQuery` logic to handle `ExitCode`, `MinDuration`, etc.
    - [x] Verify query performance with `EXPLAIN QUERY PLAN` on sample data (manual check).
- [x] Task: Update `pkg/history` public API 24bcc1a
    - [x] Update `GetCommands` to pass through new options.
    - [x] Update unit tests in `pkg/history/history_test.go`.
- [x] Task: Conductor - User Manual Verification 'Query Engine Enhancements' (Protocol in workflow.md) be780d7

## Phase 2: CLI Command Updates [checkpoint: 2212813]
- [x] Task: Update `rig history query` command 0450bc7
    - [x] Write tests for flag parsing in `cmd/history_test.go`.
    - [x] Add flags: `--exit-code`, `--min-duration`, `--ticket` to `cmd/history.go`.
    - [x] Connect flags to `pkg/history` query engine.
- [x] Task: Update `rig timeline` command filtering c8a7e4f
    - [x] Write tests ensuring timeline filters are correctly passed.
    - [x] Add support for filtering timeline generation (e.g., "show me only failed commands in the timeline").
- [x] Task: Conductor - User Manual Verification 'CLI Command Updates' (Protocol in workflow.md) 2212813

## Phase 3: Timeline Formatting & Output
- [x] Task: Design new Timeline Markdown template c9a0824
    - [x] Create `pkg/history/template.go` or similar.
    - [x] Define structs for `TimelineSection`, `TimelineItem`.
- [x] Task: Implement Formatter Logic c9a0824
    - [x] Write tests: Input `[]Command`, Output `string` (Markdown).
    - [x] Implement grouping logic (by time or session).
    - [x] Implement styling (icons for success/fail, duration strings).
- [ ] Task: Integrate Formatter into `rig timeline`
    - [ ] Replace existing raw output with new formatted output.
    - [ ] Update end-to-end tests for `rig timeline`.
- [ ] Task: Conductor - User Manual Verification 'Timeline Formatting & Output' (Protocol in workflow.md)
