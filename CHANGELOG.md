# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-01-07

### Changed

- a8fd2cb: **BREAKING**: Environment variables now require `SRE_` prefix to prevent config pollution from system variables like `TMUX`

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

- c61089f: **BREAKING**: Rename 'sre init' command to 'sre work'

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

[0.4.0]: https://github.com/thoreinstein/sre/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/thoreinstein/sre/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/thoreinstein/sre/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/thoreinstein/sre/releases/tag/v0.1.0
