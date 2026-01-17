# Specification: Refine History Analysis

## Context
The current command history analysis capabilities are limited. Users need more powerful filtering options (e.g., by exit code, duration, specific time ranges) and a more readable, formatted timeline export to effectively debrief and document their work sessions.

## Goals
1.  **Enhanced Filtering:** Implement robust filtering for command history queries, including:
    -   Exit code (success/failure)
    -   Duration (e.g., commands taking > N seconds)
    -   Time ranges (start/end)
    -   Session context
2.  **Improved Output:** Redesign the `rig timeline` output to be more visually distinct and informative, grouping related commands or highlighting key events.
3.  **Performance:** Ensure queries remain fast even with large history databases.

## Functional Requirements
-   [ ] `rig history query` accepts new flags: `--exit-code`, `--min-duration`, `--session-id`.
-   [ ] `rig timeline` generates Markdown with distinct visual styles for:
    -   Failed commands (e.g., red text or specific icon)
    -   Long-running commands (e.g., duration badge)
    -   Context switches (directory changes)
-   [ ] `rig timeline` output includes a summary section (total commands, success rate, total duration).

## Technical Requirements
-   **Language:** Go (1.25.5)
-   **Database:** SQLite (using `modernc.org/sqlite`)
-   **Libraries:** `cobra` for flags, `viper` for config.
-   **Testing:** Unit tests for new query builders and formatters. Integration test with a sample SQLite DB.

## UX Design
-   **Query:** `rig history query --failed-only --since "1 hour ago"`
-   **Timeline:**
    ```markdown
    ### 10:00 AM - Setup (15m)
    - ✅ `git checkout -b feature/new-ui`
    - ❌ `npm run test` (Failed, 2s)
    - ✅ `npm install` (1m 20s)
    ```
