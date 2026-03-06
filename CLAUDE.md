# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

Requires **Go 1.25.5+** (see `go.mod`).

```bash
# Build
go build -o rig .

# Test
go test ./...                          # All tests
go test -v ./...                       # Verbose
go test -race ./...                    # Race detection (used in CI)
go test -run TestFunctionName ./pkg/... # Single test
go test -tags integration ./...        # Integration tests (require live services)

# Lint
golangci-lint run --timeout=5m         # Lint (matches CI)
golangci-lint fmt ./...                # Format (gofmt + gci import ordering)

# Protobuf & Mocks
make generate                          # Generate Go code from .proto files
make generate-mocks                    # Generate mocks using mockery (v3)
buf lint                               # Lint proto files

# Pre-commit (mandatory before push)
pre-commit run --all-files
```

## Architecture

Rig is a Go CLI tool for developer workflow automation (git worktrees, tmux sessions, notes, JIRA/Beads integration, AI providers).

**Two-layer architecture:**
- `cmd/` — Thin CLI command handlers (Cobra). Parse input, delegate to `pkg/`, format output.
- `pkg/` — All business logic. Key packages:

| Package | Purpose |
|---------|---------|
| `api` | Protobuf gRPC service definitions (v1 API) |
| `bootstrap` | CLI initialization: flag pre-parsing, config loading, plugin registration |
| `config` | 5-tier layered config (Flags > Env > Project > User > Defaults), TOML format |
| `daemon` | Background daemon for long-running operations |
| `debrief` | Post-workflow summary and analytics |
| `discovery` | Service and plugin discovery mechanisms |
| `errors` | Domain error types (see below) |
| `git` | Worktree creation, branch management |
| `github` | GitHub API via go-github SDK |
| `ai` | AI provider abstraction (Anthropic, Gemini, Groq, Ollama) |
| `history` | SQLite command history queries (zsh-histdb/atuin) |
| `jira`, `beads` | Issue tracker integrations (API and CLI modes) |
| `notes` | Markdown note templates (Obsidian-compatible) |
| `obsidian` | Obsidian vault integration |
| `orchestration` | DAG workflow engine backed by Dolt (MySQL protocol) |
| `plugin` | gRPC plugin lifecycle, discovery from `~/.config/rig/plugins/` |
| `project` | Project root discovery via marker files (.git, .beads, go.mod) |
| `sdk` | Plugin SDK interfaces and handshake protocol |
| `tmux` | Session automation |
| `ui` | UI components and interaction handlers |
| `workflow` | Workflow automation engine |

**Plugin system:** Plugins communicate over gRPC via Unix Domain Sockets. Discovery is restricted to `~/.config/rig/plugins/`. Plugins negotiate capabilities (command, assistant, node executor) during handshake.

## Code Conventions

**Imports** — Three groups enforced by `gci` linter:
```go
import (
    "standard/library"

    "third/party"

    "thoreinstein.com/rig/pkg/..."
)
```

**Errors** — Use `github.com/cockroachdb/errors` (aliased as `rigerrors` when colliding with stdlib `errors`). Never use `github.com/pkg/errors`. Domain types in `pkg/errors`: `ConfigError`, `GitHubError`, `AIError`, `JiraError`, `BeadsError`, `WorkflowError`, `CapabilityError`, `PluginError`, `DaemonError`.

**Naming** — Files: `snake_case.go`. Directories: `kebab-case`. Enums: `SCREAMING_SNAKE`. Go types/functions follow standard Go conventions.

**Blocked packages** (enforced by `gomodguard`):
- `github.com/pkg/errors` → use `github.com/cockroachdb/errors`
- `gopkg.in/yaml.v2` or `v3` → use `go.yaml.in/yaml/v3`
- `github.com/golang/protobuf` → use `google.golang.org/protobuf`
- `golang.org/x/net/context` → use stdlib `context`
- `math/rand` → use `math/rand/v2`

## Testing Conventions

- **Table-driven tests are mandatory** for all Go logic
- Tests live alongside code (`*_test.go` in same package)
- Use `t.TempDir()`, `t.Setenv()`, `t.Context()` (enforced by `usetesting` linter)
- **Mocking**: Mock external dependencies via interfaces.
    - Prefer **generated mocks** using `mockery` (run `make generate-mocks`).
    - Mocks are stored in `mocks/` subdirectories adjacent to the interface.
    - Hand-written mocks are acceptable for legacy code or complex logic.
- **Assertions**: Use `github.com/stretchr/testify/require` (or `assert`) to supplement standard library testing. Table-driven tests and `t.Run` subtests remain mandatory.
- Integration tests use `//go:build integration` build tag
- Unset `GIT_*` env vars in tests that create temporary git repos (pre-commit injects these)
- Reset global caches (`project.ResetCache()`, `resetConfig()`) between tests to prevent cross-test contamination

## Key Patterns

- **Functional options** for constructors (`...Option` parameters)
- **`sync.Once` lazy init** for AI providers (avoids context.Context in constructors)
- **Daemon-first execution**: Try daemon, fall back to direct on transport failure
- **Unique UDS paths**: Truncated UUIDs (8 chars) for plugin socket paths to stay under 104-char Darwin limit
- **Protobuf tag immortality**: Never reuse or repurpose proto field tags; use `reserved` for removed fields

## CI Pipeline

GitHub Actions on push/PR to `main` or `rig-v1`:
1. **Lint**: `golangci-lint run` (v2.9.0, 5m timeout)
2. **Test**: `go test -v -race ./...` (requires gcc and tmux)
3. **Build**: `go build -v -o rig .`

## Git Workflow

- Base branch: `rig-v1`
- Config: `~/.config/rig/config.toml` (TOML only, strictly enforced)
- Worktree-friendly: pre-commit uses `GOFLAGS=-buildvcs=false`
