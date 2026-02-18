# Code Style & Conventions

## Language: Go (1.25+)

## Naming
- **Files**: `snake_case.go`
- **Directories**: `kebab-case` (e.g., `pkg/api-client`)
- **Go Fields/Vars**: `CamelCase` (Standard Go)
- **Protobuf**: Standard proto3 style

## Testing
- **Mandatory**: New features must have tests.
- **Pattern**: Table-driven tests are preferred.
- **Mocking**: Use interfaces for external dependencies (Git, JIRA, Tmux).

## Architecture
- **CLI Logic**: Keep `cmd/` minimal. Delegate business logic to `pkg/`.
- **Error Handling**: Use `github.com/cockroachdb/errors` for rich error wrapping.
- **Configuration**: Use `viper` for config loading.

## Documentation
- **Comments**: Focus on *why*, not *what*.
- **Artifacts**: Store plans and designs in Obsidian (`working/rig/plans/`), not the repo.
- **Context**: `GEMINI.md` is the source of truth for architectural decisions.
