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
  - Use interfaces for mocking external dependencies (Git, JIRA, Tmux, Plugin Executors).
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
- **Plugin-First Architecture:** AI providers can be offloaded to standalone gRPC plugins implementing the `AssistantService`. This allows for high-performance gRPC streaming and decoupled provider lifecycles.
- **Plugin Assistant Provider:** A specialized `ai.Provider` implementation (`PluginAssistantProvider`) acts as a gRPC client to "Assistant" plugins, translating between internal `ai` types and gRPC proto messages.

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
- **Sidecar Manifests:** Use `<plugin>.manifest.yaml` or `manifest.yaml` (inside subdirectories) to provide metadata (Name, Version, Requirements) without executing the plugin binary. Metadata is optional; directory-based plugins with valid executables are discovered even if a manifest is missing.
- **Lazy Validation:** Perform compatibility validation during the discovery/listing phase using the current binary's version to ensure accurate SemVer checks.
- **Unix-Only Execution:** Plugins must be Unix executable files (marked with execute bits). Non-executable files and those with unknown extensions are ignored. Subdirectories must contain at least one executable file to be considered valid plugins.
- **Manifest Resolution:** Sidecar manifests are matched against the logical plugin name (stripping extensions like `.sh` or `.py`).
- **Fail-Fast Plugin Initialization Pattern**: Establish a mandatory validation gate during the initial handshake. The `Manager` MUST call `ValidateCompatibility` immediately after the handshake and reject (Stop) any plugin returning `StatusIncompatible` or `StatusError` before it enters the active pool.

### Workflow Traps
- **Sparse-Checkout Staging:** In a `git sparse-checkout` environment, new files must be staged using `git add --sparse <path>` if they fall outside the current sparse index definition.
- **Surfacing Metadata Errors:** If a plugin's manifest file exists but is malformed, report an error rather than silently ignoring it. This prevents bypassing version checks due to configuration errors. Ensure consistent error reporting across all discovery paths (e.g., both single-binary and directory-based plugins).
- **Automated Reviewer Naming Conflicts**: Conflicting bot suggestions (e.g., snake_case vs. camelCase) should be resolved by prioritizing Go idioms and established codebase patterns over generic bot rules.

## API & Plugin Architecture (gRPC)

### Governance (v1)
- **Versioning:** Strict `Major.Minor.Patch` schema enforced via package names (e.g., `rig.v1`).
- **Deprecation:** 3-version policy: Deprecate (N) -> Warn (N+1) -> Remove (N+2).
- **Breaking Changes:** Only allowed in new major versions (e.g., `v2`).

### Buf Configuration Patterns
- **Standard Linting:** Use `lint.use: [STANDARD]` in `buf.yaml`. Avoid legacy `DEFAULT`.
- **Package Directory Match:** Explicitly exclude `PACKAGE_DIRECTORY_MATCH` lint rule if proto package structure (e.g., `rig.v1`) differs from file path (e.g., `pkg/api/v1`).
- **Dependency Traps:** NEVER use raw git URLs (e.g., `github.com/...`) in `buf.yaml` `deps`. Use valid Buf Schema Registry module references (e.g., `buf.build/protocolbuffers/protobuf`).

### Plugin Execution & Handshake
- **Unique IPC Endpoints:** Always generate unique Unix Domain Socket (UDS) paths for concurrent plugin instances. Use truncated UUIDs (first 8 chars) to avoid exceeding the 104-108 character limit for socket paths on Darwin/Linux.
- **Environment Handshake:** Pass the socket path to the plugin via the `RIG_PLUGIN_ENDPOINT` environment variable.
- **Readiness Polling:** The host must poll for the socket file and verify readiness with a `net.Dial` before attempting the gRPC `Handshake` RPC.
- **Insecure Local Transport:** Use `insecure.NewCredentials()` for gRPC over UDS, as communication is restricted to the local host.
- **Total State Reset Pattern:** Handshake logic must explicitly clear all internal state (capabilities, versions) if corresponding response fields are absent. This prevents "ghost state" where Rig retains stale information from previous sessions.
- **Compatibility Translation Layer:** The host client should act as a translation shim, prioritizing modern structured fields (e.g., `plugin_semver`) but providing fallbacks/translation for legacy tags (e.g., `plugin_version`) to maintain wire compatibility during V1 migration.

#### Capability Modeling & AI
- **Assistant Capability:** Plugins advertising the `assistant` capability must implement the `AssistantService` (defined in `assistant.proto`) to provide AI completion and streaming services.
- **Discovery & Lifecycle:** The host's `PluginManager` discovers plugins via `Scanner`, starts them via `Executor`, and manages the connection state. Clients for specific capabilities (like `AssistantServiceClient`) are lazily initialized from the shared gRPC connection.
- **Dynamic Routing:** The AI factory (`NewProviderWithManager`) can route requests to specific assistant plugins by specifying the `plugin` provider and the plugin's name in the configuration.

### Protobuf & API Evolution
- **Tag Immortality:** Field tags in protobuf messages are permanent identities. Never reuse or repurpose a tag number for a different semantic meaning, even if renamed. Use `reserved` for removed tags to prevent future accidental reuse.
- **Hybrid Snake_Case Mapping:** Follow Go naming idioms for internal struct fields (e.g., `APIVersion` with all-caps initialisms) while using explicit struct tags (`json:"api_version"`) to satisfy project-wide `snake_case` requirements for external serialization.
- **API Standardisation:** Prefer industry-standard protocols (e.g., `grpc.health.v1`) over custom implementations for common infrastructure needs like health monitoring.
- **Fail-Fast Mocking:** When mocking complex gRPC interfaces, implementations MUST fail loudly (return error) for unset function fields rather than returning `nil`.
- **Stream Naming Inversion:** For bi-directional streams, name messages by *direction* (Request=Client->Server, Response=Server->Client) rather than payload semantics, but document the inversion clearly.
