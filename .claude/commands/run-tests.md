---
description: Run targeted tests for recently changed packages
---

Identify which Go packages have been modified (via `git diff --name-only` for both staged and unstaged changes) and run tests only for those packages.

Steps:
1. Run `git diff --name-only` and `git diff --cached --name-only` to find changed .go files
2. Extract the unique package directories from the changed files
3. Run `go test -v -race` on each affected package
4. Report a summary: which packages were tested, pass/fail status, and any failures

If no Go files have changed, say so and ask if the user wants to run the full suite instead.

Use $ARGUMENTS as an optional package path override (e.g., `/run-tests ./pkg/config/...`).
