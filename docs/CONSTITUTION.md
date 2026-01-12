# rig Project Constitution

## Vision

Streamline developer workflows by making repetitive tasks efficient and consistent.

## Target Users

- Individual developers
- SRE teams
- DevOps engineers
- Open source community

## Success Metrics

- Reduce task completion time for common developer workflows

## Core Principles

- **Simplicity first** - Prefer simple, obvious solutions over clever or complex ones

## Boundaries

This project will NOT:

- Have a GUI - CLI only, always
- Require cloud connectivity - core features must work offline
- Become bloated - single binary, minimal dependencies

## Technical Decisions

### Language & Framework
- **Go** with **Cobra** CLI framework
- Standard library testing (`go test`)
- Strict linting via `golangci-lint`

### Code Style
- File naming: `snake_case.go`
- Database fields: `snake_case` (read-only access)
- Enums: `SCREAMING_SNAKE_CASE`

### Development Workflow
- Test: `go test ./...`
- Lint: `golangci-lint run`
- Build: `go build`
