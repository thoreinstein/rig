# Rig - Developer Workflow Automation CLI

## Project Overview

**Rig** is a modern, extensible Go CLI tool designed to streamline developer workflows. It replaces complex bash scripts with a maintainable, feature-rich application that automates:
- **Git Worktrees:** Automatic isolated workspaces per ticket/branch.
- **Tmux Sessions:** Configurable multi-window terminal sessions.
- **Documentation:** Markdown note creation (Obsidian-style) with JIRA and Beads integration.
- **History Tracking:** Command history analysis and timeline export using SQLite.
- **AI Integration:** Support for various AI providers (Anthropic, Gemini, Groq, Ollama) for workflow assistance.

## Key Technologies

- **Language:** Go (1.25+)
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **Configuration:** [Viper](https://github.com/spf13/viper)
- **Database:** SQLite (modernc.org/sqlite)
- **Integrations:** Git, Tmux, JIRA, Beads, Obsidian, GitHub
- **Error Handling:** [cockroachdb/errors](https://github.com/cockroachdb/errors)

## Directory Structure

- `cmd/`: CLI command implementations (e.g., `work.go`, `hack.go`, `session.go`).
- `pkg/`: Core logic packages.
  - `pkg/git/`: Git worktree and repository management.
  - `pkg/tmux/`: Tmux session automation.
  - `pkg/jira/`: JIRA integration (API & CLI).
  - `pkg/beads/`: Beads issue tracker integration.
  - `pkg/history/`: Command history tracking (zsh-histdb/atuin).
  - `pkg/config/`: Configuration handling and security warnings.
  - `pkg/notes/`: Markdown note templates and management.
  - `pkg/ai/`: AI provider implementations.
  - `pkg/github/`: GitHub API and CLI client.
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
golangci-lint run      # Run linters
```

## Configuration

Rig uses a TOML configuration file, typically located at `~/.config/rig/config.toml`. It also supports repository-local overrides via `.rig.toml`.

### Key Config Sections
- **[notes]**: Path to Obsidian/Markdown notes and templates.
- **[git]**: Base branch configuration.
- **[jira]**: JIRA credentials and mode (API vs ACLI).
- **[beads]**: Beads integration settings.
- **[tmux]**: Session window layouts and commands.
- **[history]**: Database path for command history.
- **[ai]**: AI provider and model settings.

## Development Conventions

- **Style:** Follow standard Go idioms.
- **Naming:**
  - Fields/Files: `snake_case`
  - Directories: `kebab-case`
  - Enums: `SCREAMING_SNAKE`
- **Testing:**
  - Mandatory for new features.
  - Prefer table-driven tests.
  - Use interfaces for mocking external dependencies (Git, JIRA, Tmux).
- **Architecture:** Keep CLI logic in `cmd/` minimal; delegate business logic to `pkg/`.
- **Linting:** Strict mode using `golangci-lint`.

## Key Commands

- `rig work <ticket>`: Start a complete workflow (Worktree + Tmux + Note).
- `rig hack <name>`: Start a lightweight non-ticket workflow.
- `rig list`: Show active worktrees and sessions.
- `rig clean`: Remove old worktrees and sessions.
- `rig sync <ticket>`: Update notes with latest JIRA/Beads info.
- `rig timeline <ticket>`: Export command history for a ticket.
- `rig history query`: Search through command history.

## Lessons Learned & Architectural Truths

### AI Provider Patterns
- **Lazy Initialization:** Use `sync.Once` and an `init(ctx)` method for providers requiring SDK setup or context. This avoids passing `context.Context` to constructors and defers initialization until first use.
- **Interface-Based Mocking:** Test AI providers by injecting and mocking the underlying SDK model interfaces (e.g., Genkit's `ai.Model`). This enables fast, deterministic testing of message mapping and token usage.
- **Translation Layer:** Maintain internal `ai.Message` and `ai.Response` abstractions. Map these to SDK-specific types within the provider implementation to protect the codebase from underlying SDK breaking changes.

### AI Configuration
Providers are configured in `~/.config/rig/config.toml`.

#### Gemini (Google AI)
Uses the native Genkit for Go SDK.
```toml
[ai]
provider = "gemini"
gemini_model = "gemini-1.5-flash" # Optional
gemini_api_key = "your-api-key"   # Or use GOOGLE_GENAI_API_KEY
```

#### Anthropic / Groq
```toml
[ai]
provider = "anthropic"
api_key = "your-api-key" # Or use ANTHROPIC_API_KEY / GROQ_API_KEY
```

### Configuration Traps
- **Isolated Secret Resolution:** Use isolated resolution functions for each provider to prevent "cross-provider contamination" (e.g., using an Anthropic key for Gemini).
- **Security Warning Accuracy:** When implementing security warnings for config-stored secrets, ensure all valid environment variable sources (e.g., `RIG_AI_*`) are checked to avoid false positives.

### Architectural Decisions
- **SDK-First:** Prefer official Go SDKs (like Genkit for Google AI) over CLI wrappers for stability and robust streaming support.
- **Safe Deprecation:** Use non-blocking configuration warnings (via `CheckSecurityWarnings`) instead of silent removal when deprecating keys to ensure a smooth user migration.

### Documentation Conventions
- **Historical Accuracy:** Never modify old release notes or historical documentation to reflect current state. Always treat past records as immutable snapshots.
- **Consolidated AI Context:** Keep AI provider configuration examples and architectural truths in `GEMINI.md` to provide a single source of truth for future agent sessions.

### Plugin Architecture & Discovery
- **Safe Path Discovery:** Restrict plugin discovery to `~/.config/rig/plugins` to maintain system security.
- **Sidecar Manifests:** Use `<plugin>.manifest.yaml` or `manifest.yaml` (inside subdirectories) to provide metadata (Name, Version, Requirements) without executing the plugin binary.
- **Lazy Validation:** Perform compatibility validation during the discovery/listing phase using the current binary's version to ensure accurate SemVer checks.
- **Cross-Platform Executability:** Detect plugins across platforms by checking for both common executable extensions (.exe, .bat, .cmd) and Unix execute bits.

### Workflow Traps
- **Sparse-Checkout Staging:** In a `git sparse-checkout` environment, new files must be staged using `git add --sparse <path>` if they fall outside the current sparse index definition.
- **Platform-Dependent Testing:** Ensure platform-sensitive logic (like file detection) is "simulatable" in unit tests by avoiding strict OS-gating in helpers, allowing Windows logic to be tested on Unix hosts.
- **Defensive Environment Resolution:** Always handle errors from system path lookups (e.g., `os.UserHomeDir()`). Failing to anchor to the home directory can cause silent drift into the current working directory, breaking "Safe Path" guarantees.
- **Identity Theft (Global Fallbacks):** Avoid global metadata fallbacks (like a root `manifest.yaml`) in shared plugin directories. Use strictly scoped sidecars (`<name>.manifest.yaml`) to prevent binaries from inheriting incorrect metadata.
- **Persist-Credentials Trade-off:** Setting `persist-credentials: false` in `actions/checkout` breaks subsequent authenticated Git checks (e.g., `git ls-remote`). Set to `true` if the job needs to talk back to `origin`.

## API & Plugin Architecture (gRPC)

### Governance (v1)
- **Versioning:** Strict `Major.Minor.Patch` schema enforced via package names (e.g., `rig.v1`).
- **Deprecation:** 3-version policy: Deprecate (N) -> Warn (N+1) -> Remove (N+2).
- **Breaking Changes:** Only allowed in new major versions (e.g., `v2`).

### Buf Configuration Patterns
- **Standard Linting:** Use `lint.use: [STANDARD]` in `buf.yaml`. Avoid legacy `DEFAULT`.
- **Package Directory Match:** Explicitly exclude `PACKAGE_DIRECTORY_MATCH` lint rule if proto package structure (e.g., `rig.v1`) differs from file path (e.g., `pkg/api/v1`).
- **Dependency Traps:** NEVER use raw git URLs (e.g., `github.com/...`) in `buf.yaml` `deps`. Use valid Buf Schema Registry module references (e.g., `buf.build/protocolbuffers/protobuf`).
