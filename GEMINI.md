# Rig - Developer Workflow Automation CLI

## Project Overview

**Rig** is a modern, extensible Go CLI tool designed to streamline developer workflows. It replaces complex bash scripts with a maintainable, feature-rich application that automates:
- **Git Worktrees:** Automatic isolated workspaces per ticket.
- **Tmux Sessions:** Configurable multi-window terminal sessions.
- **Documentation:** Markdown note creation (Obsidian-style) with JIRA integration.
- **History Tracking:** Command history analysis and timeline export using SQLite.

## Key Technologies

- **Language:** Go (1.25+)
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **Configuration:** [Viper](https://github.com/spf13/viper)
- **Database:** SQLite (modernc.org/sqlite)
- **Integrations:** Git, Tmux, JIRA (via API or ACLI), Obsidian, GitHub

## Directory Structure

The project root contains the following key components:

- `cmd/`: CLI command implementations (e.g., `work.go`, `hack.go`, `session.go`).
- `pkg/`: Core logic packages.
  - `pkg/git/`: Git worktree and repository management.
  - `pkg/tmux/`: Tmux session automation.
  - `pkg/jira/`: JIRA integration (API & CLI).
  - `pkg/history/`: Command history tracking (zsh-histdb/atuin).
  - `pkg/config/`: Configuration handling.
  - `pkg/obsidian/`: Markdown note templates and management.
- `main.go`: Application entry point.
- `project.yaml`: Project metadata and governance.

## Building and Running

### Build
```bash
go build -o rig
```

### Run
```bash
./rig --help
```

### Test
The project emphasizes comprehensive test coverage using table-driven tests and mocks.
```bash
go test ./...          # Run all tests
go test -v ./...       # Verbose output
golangci-lint run      # Run linters (if installed)
```

## Configuration

Rig uses a TOML or YAML configuration file, typically located at `~/.config/rig/config.toml`.
Run `./rig config --init` to generate a default configuration.

### Key Config Sections
- **[vault]**: Path to Obsidian/Markdown notes.
- **[repository]**: Git repository details (owner, name, base path).
- **[jira]**: JIRA credentials and mode (API vs ACLI).
- **[tmux]**: Session window layouts and commands.
- **[history]**: Database path for command history.

## Development Conventions

- **Style:** Follow standard Go idioms.
- **Naming:**
  - Fields/Files: `snake_case`
  - Directories: `kebab-case`
  - Enums: `SCREAMING_SNAKE`
- **Testing:**
  - Prefer table-driven tests.
  - Use interfaces for mocking external dependencies (Git, JIRA, Tmux).
- **Architecture:** Keep CLI logic in `cmd/` minimal; delegate business logic to `pkg/`.
- **Linting:** Strict mode using `golangci-lint`.

## Key Commands

- `rig work <ticket>`: Start a complete workflow (Worktree + Tmux + Note).
- `rig hack <name>`: Start a lightweight non-ticket workflow.
- `rig clean`: Remove old worktrees and sessions.
- `rig list`: Show active worktrees and sessions.
- `rig sync <ticket>`: Update notes with latest JIRA info.
- `rig timeline <ticket>`: Export command history for a ticket.
