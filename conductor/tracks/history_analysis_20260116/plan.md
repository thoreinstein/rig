# Implementation Plan - Refine History Analysis

## Phase 1: Query Engine Enhancements
- [ ] Task: Define new query parameters in `pkg/history/types.go`
    - [ ] Create `QueryOptions` struct with `ExitCode`, `MinDuration`, `SessionID`.
    - [ ] Update `Search` interface signature to accept `QueryOptions`.
- [ ] Task: Implement SQL query builder updates in `pkg/history/sqlite.go`
    - [ ] Write tests for `BuildQuery` ensuring correct WHERE clauses for new filters.
    - [ ] Implement `BuildQuery` logic to handle `ExitCode`, `MinDuration`, etc.
    - [ ] Verify query performance with `EXPLAIN QUERY PLAN` on sample data (manual check).
- [ ] Task: Update `pkg/history` public API
    - [ ] Update `GetCommands` to pass through new options.
    - [ ] Update unit tests in `pkg/history/history_test.go`.
- [ ] Task: Conductor - User Manual Verification 'Query Engine Enhancements' (Protocol in workflow.md)

## Phase 2: CLI Command Updates
- [ ] Task: Update `rig history query` command
    - [ ] Write tests for flag parsing in `cmd/history_test.go`.
    - [ ] Add flags: `--exit-code`, `--min-duration` to `cmd/history.go`.
    - [ ] Connect flags to `pkg/history` query engine.
- [ ] Task: Update `rig timeline` command filtering
    - [ ] Write tests ensuring timeline filters are correctly passed.
    - [ ] Add support for filtering timeline generation (e.g., "show me only failed commands in the timeline").
- [ ] Task: Conductor - User Manual Verification 'CLI Command Updates' (Protocol in workflow.md)

## Phase 3: Timeline Formatting & Output
- [ ] Task: Design new Timeline Markdown template
    - [ ] Create `pkg/history/template.go` or similar.
    - [ ] Define structs for `TimelineSection`, `TimelineItem`.
- [ ] Task: Implement Formatter Logic
    - [ ] Write tests: Input `[]Command`, Output `string` (Markdown).
    - [ ] Implement grouping logic (by time or session).
    - [ ] Implement styling (icons for success/fail, duration strings).
- [ ] Task: Integrate Formatter into `rig timeline`
    - [ ] Replace existing raw output with new formatted output.
    - [ ] Update end-to-end tests for `rig timeline`.
- [ ] Task: Conductor - User Manual Verification 'Timeline Formatting & Output' (Protocol in workflow.md)
