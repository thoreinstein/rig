# Task Completion Protocol

Before marking a task as DONE:

1.  **Verify Code**:
    - Run `go build` to ensure compilation.
    - Run `golangci-lint run` to check for style/bugs.
    - Run `go test ./...` (or specific package tests) to verify functionality.

2.  **Verify Docs**:
    - Update `README.md` if user-facing behavior changed.
    - Update `GEMINI.md` if architectural decisions were made.
    - Save implementation summaries to Obsidian (`working/rig/summaries/`).

3.  **Commit**:
    - Stage changes: `git add <files>`
    - Check diff: `git diff --staged`
    - Commit: `git commit -m "feat: <description>"` (Conventional Commits)

4.  **Sync**:
    - Update Beads: `bd close <ticket-id>`
    - Push: `git push`
    - Sync Beads: `bd sync`
