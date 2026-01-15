# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.9.0] - 2026-01-15

### Added

- 9c983d6: Add pkg/beads with detection and client interface
  - BeadsClient interface and CLIClient implementation
  - Project detection via `.beads/beads.jsonl`
- 40379f2: Implement TicketRouter for beads vs JIRA routing
  - Alphanumeric ticket IDs (rig-abc123) → beads
  - Numeric ticket IDs (PROJ-123) → JIRA
- 2cb8b06: Integrate beads status update into rig work command
- c05b7ea: Integrate AI provider into workflow engine for debrief
- a3108c4: Handle branch deletion via API to avoid worktree conflicts

### Fixed

- 7e54767: Fix Beads detection to use correct filename (beads.jsonl)
- a4a75f5: Handle TicketSourceUnknown in preflight switch
- 4e2eec4: Fix preflight empty FailureReason when Jira unavailable
- 4696bd9: Fix preflight error guidance to be context-aware
- 7d5cf7f: Fix undefined cmd variable in runPRMerge
- d971bcd: Improve JIRA client initialization error message
- f0d94ee: Fix pr merge attempting Jira fetch for beads tickets
- 73dfeb1: PR merge cleanup

### Tests

- 88f7052: Add explicit AI config initialization in router tests
- 2a79819: Add comprehensive unit tests for BeadsError type

## [0.8.0] - 2026-01-14

### Changed

- 7988a30: **BREAKING**: Notes are now created by default for `hack` and `work` commands
  - The `--notes` flag has been removed from `hack`
  - Use `--no-notes` to skip note creation
  - `hack` now updates daily notes (matching `work` behavior)

### Fixed

- e86e9d1: Fix `rig pr create` failing in non-interactive mode
  - `--body` flag is now always passed to `gh pr create`
  - Resolves rig-cge, rig-a6t

## [0.7.1] - 2026-01-14

### Enhanced

- fdfe6d7: Accept beads-style alphanumeric ticket identifiers
  - Ticket parsing now accepts alphanumeric suffixes (e.g., `rig-2o1`, `beads-xyz`)
  - All existing numeric formats (`PROJ-123`) continue to work unchanged
  - Affects `rig work` and `rig sync` commands

## [0.7.0] - 2026-01-14

### Added

- e5a3dc4: Add Ollama provider for local LLM inference
  - Local AI inference via Ollama `/api/chat` endpoint
  - No API key required; works offline
- 6588683: Add native GitHub API client
  - Replaces `gh` CLI wrapper with `google/go-github/v68`
  - Multi-layer auth: env vars, config, OAuth device flow, gh fallback
  - Secure token caching via OS keychain
- 54e86d2: Add `rig pr` command family
  - `rig pr create` - Create PR from current branch
  - `rig pr view` - View PR details
  - `rig pr list` - List PRs with filtering
  - `rig pr merge` - Full merge workflow with AI debrief
- d735d11: Add merge workflow engine and AI debrief
  - 5-step workflow: Preflight → Gather → Debrief → Merge → Closeout
  - Checkpoint system for resuming interrupted workflows
- 88abf67: Add GitHub, AI, and Jira workflow integration packages
- 6344bf5: Add GitHub, AI, workflow configuration and error framework
- 0aec65d: Add `--skip-approval` flag for self-authored PRs

### Fixed

- 5878f2a: Fix Copilot review comments (rig-oxt, rig-4sd)
- 8517449: Fix GitHub PR reviews response parsing
- 29d8cbb: Fix pre-commit hooks for git worktree compatibility

### Changed

- **Internal API**: `github.Client.ListPRs` now accepts author filter parameter

### Dependencies

- 667e961: Bump anchore/sbom-action from 0.21.0 to 0.21.1
- 5840c05: Bump modernc.org/sqlite from 1.42.2 to 1.43.0

## [0.6.0] - 2026-01-12

### Added

- a04d09d: Add direct Jira REST API integration with `mode: "api"` configuration
  - Native Jira Cloud REST API v3 support without external CLI dependencies
  - Automatic rate limiting with exponential backoff and jitter
  - Respects `Retry-After` headers from Jira API
  - `JIRA_TOKEN` environment variable support (takes precedence over config)
- a216bee: Add custom field support in Jira details section
  - Configure field mappings via `jira.custom_fields` in config
  - Supports string, number, object, and array field types
  - Display custom fields (e.g., story points) in ticket notes

### Changed

- cec5f52: Update README description to reflect developer focus
- 5f62841, 6aa971e: Refactor Jira client to use factory pattern for mode selection

### Internal

- 3d3dc80: Setup parade project scaffolding
- 6e2ff44: Update beads issue tracking configuration

## [0.5.0] - 2026-01-08

### Changed

- 736377b: **BREAKING**: Rename Go module from `thoreinstein.com/sre` to `thoreinstein.com/rig`
- 736377b: **BREAKING**: Rename binary from `sre` to `rig`
- 736377b: **BREAKING**: Move config directory from `~/.config/sre/` to `~/.config/rig/`
- 8226d2f: Update all documentation for rig rename
- 5bdf61d: Update build configuration (CI, golangci, goreleaser) for rig rename

### Fixed

- bc1cb9e: Fix lint nits across command files

## [0.4.1] - 2026-01-08

### Fixed

- 96a07c3: Fix tmux window indexing to respect base-index setting

### CI

- cfbb626: Install tmux in CI workflow for integration tests

## [0.4.0] - 2026-01-07

### Changed

- a8fd2cb: **BREAKING**: Environment variables now require `RIG_` prefix to prevent config pollution from system variables like `TMUX`

### Fixed

- a8fd2cb: Fix tmux configuration corruption when running inside existing tmux sessions

### Tests

- 1fef5a4: Refactor clone tests to avoid network dependency
- 1fef5a4: Add tmux session window creation integration test

### Developer Experience

- aad7d12: Remove go-test from pre-commit hooks (CI handles test execution)

## [0.3.0] - 2026-01-06

### Added

- 7b057e5: Add clone command to CLI
- 1e1211f: Add clone command core functionality

## [0.2.0] - 2026-01-06

### Added

- 40ead95: Add auto-repair for missing fetch refspec in bare repos
- 78d7129: Add beads issue tracking infrastructure
- 0e84dc9: Add release documentation for v0.1.0
- 34e4a5c: Add environment variable-based tmux socket isolation for tests

### Changed

- c61089f: **BREAKING**: Rename 'init' command to 'rig work'

### Tests

- 0d22c9e: Add TestMain to clean up tmux sessions after tests

### Dependencies

- ddb3360: Bump modernc.org/sqlite from 1.41.0 to 1.42.2

## [0.1.0] - 2025-12-29

### Added

- 726f978: Add self-update command for GitHub releases
- 04f1858: Add multi-repo support and new workflow commands
- d776daa: Add Homebrew tap and macOS installation docs
- 9314f79: Add keyless Sigstore signing and SBOM generation
- 8b0b6b7: Add CI/CD pipeline with GitHub Actions and GoReleaser
- 3708712: Add Dependabot and CodeQL security scanning
- 6ad14f4: Add build and test hooks to pre-commit config
- e091f5c: Add LICENSE
- db98a9c: Add README

### Changed

- c2971bc: Harden security and migrate config to TOML format
- f8e037b: Remove C dependency in favor of native pure Go SQLite
- 462840c: Use latest Go 1.25.5
- 2c232e3: Update linter configuration
- 564bce7: Update lint and release configs
- 5915a01: Add cooldown periods to Dependabot configuration
- 5730c80: Update README with new commands, multi-repo config, and testing docs

### Fixed

- eb98f47: Fix race conditions in cmd test files
- c726210: Fix tmux windows not being created due to type mismatch
- ae8ad1b: Fix inconsistent file permissions for user data
- 6afe142: Fix GitHub Actions SHA pins to verified versions
- b2d4dbe: Fix worktree location handling
- 92ed3cb: Fix multiple bugs found during code review
- eb3b94e: Fix trailing whitespace and normalize formatting

### Security

- 3d4a724: Harden CI/CD workflows for security
- 673e8f6: Harden release workflow with least-privilege permissions
- 512c27a: Pin GitHub Actions to SHA commits for supply chain security
- f9d2174: Add command allowlist security tests for tmux sessions
- bb6aa6f: Add path traversal security tests for worktree operations

### Tests

- 0265075: Add comprehensive test coverage across all packages
- cf2bd96: Add test coverage for all cmd/ package files
- 25e2805: Add test coverage for multi-repo support and workflow commands
- 9870e19: Add mock-based unit tests for pkg/git/worktree.go
- 74fd676: Add integration tests and fix GPG signing in test fixtures
- 5ea7618: Expand test coverage for history, session, sync, timeline, update commands
- 91d92fe: Expand obsidian notes test coverage

### Dependencies

- 351eac9: Bump the actions group with 7 updates
- d505e09: Bump github.com/spf13/viper from 1.20.1 to 1.21.0
- aa8759d: Bump modernc.org/sqlite from 1.40.1 to 1.41.0
- 331620e: Bump github.com/go-viper/mapstructure/v2 from 2.2.1 to 2.4.0
- f749a0a: Bump actions/setup-go from 5 to 6
- be09b56: Bump actions/checkout from 4 to 6
- 699e774: Bump github/codeql-action from 3 to 4
- bc72a00: Bump golangci/golangci-lint-action from 6 to 9
- bbc29e2: Bump golangci/golangci-lint-action from 6 to 9

[0.9.0]: https://github.com/thoreinstein/rig/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/thoreinstein/rig/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/thoreinstein/rig/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/thoreinstein/rig/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/thoreinstein/rig/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/thoreinstein/rig/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/thoreinstein/rig/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/thoreinstein/rig/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/thoreinstein/rig/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/thoreinstein/rig/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/thoreinstein/rig/releases/tag/v0.1.0
