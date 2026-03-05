# Rig - Developer Workflow Automation CLI

## Project Overview

**Rig** is a modern, extensible Go CLI tool designed to streamline developer workflows. It replaces complex bash scripts with a maintainable, feature-rich application that automates:
- **Git Worktrees:** Automatic isolated workspaces per ticket/branch.
- **Tmux Sessions:** Configurable multi-window terminal sessions.
- **Documentation:** Markdown note creation (Obsidian-style) with JIRA and Beads integration.
- **History Tracking:** Command history analysis and timeline export using SQLite.
- **AI Integration:** Support for various AI providers (Anthropic, Gemini, Groq, Ollama) for workflow assistance.
- **Orchestration:** Configuration-driven DAG execution engine with Dolt-backed persistence.

## Key Technologies

- **Language:** Go (1.25+)
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **Configuration:** [Viper](https://github.com/spf13/viper)
- **Database:** SQLite (modernc.org/sqlite) and Dolt (embedded driver)
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
  - `pkg/bootstrap/`: Heavy CLI initialization and bootstrap logic.
  - `pkg/orchestration/`: Dolt-backed workflow persistence and DAG orchestration.
  - `pkg/knowledge/`: Unified interface for notes, Obsidian, and plugins.
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
- **[plugins]**: Plugin-specific configuration sections.

## Development Conventions

- **Style:** Follow standard Go idioms.
- **Naming:**
  - Fields/Files: `snake_case`
  - Directories: `kebab-case`
  - Enums: `SCREAMING_SNAKE`
- **Error Handling Patterns:**
  - **Fluent Error Chaining:** Custom domain errors MUST implement a `WithCause(err error) *CustomError` method to allow for readable, fluent wrapping while preserving the error chain for diagnostics.
  - **Isolated Domain Types:** Subsystems (Database, GitHub, AI) should use their own error structs (e.g., `DatabaseError`) to provide structured metadata (error codes, retryability).
- **Testing:**
  - Mandatory for new features.
  - ALWAYS use table-driven tests for all Go logic.
  - Prefer table-driven tests for comprehensive edge case coverage.
  - Use interfaces for mocking external dependencies (Git, JIRA, Tmux, Plugin Executors).
  - **gRPC Integration Testing:** New plugin capabilities MUST include a full-loop integration test using `testsdk`. Unit tests verify logic, but integration tests verify the gRPC wiring and service registration.
- **Architecture:**
  - Keep CLI logic in `cmd/` minimal (thin wrappers).
  - Delegate orchestration and business logic to `pkg/`.
  - Move heavy bootstrap and initialization logic to `pkg/bootstrap`.
  - **Tiered Provider Pattern:** Core integrations (VCS, Ticketing, Knowledge) must follow a tiered provider architecture: `Provider` interface -> `LocalProvider` (direct logic) -> `PluginProvider` (gRPC client). This ensures zero-IPC for local development while enabling isolated plugin-based backends.
  - **Secret Proxy (Host-as-Server):** Isolated plugins MUST NOT have direct access to host environment variables or keychains. Sensitive API tokens (like `JIRA_TOKEN`) must be requested via the `SecretService` gRPC proxy on the host (`RIG_HOST_ENDPOINT`), which validates access using session-scoped tokens and resolves values from per-plugin configuration (e.g. `[plugins.<name>.secrets]`).
  - **Zero-Exposure Secret Lifecycle:** To prevent accidental leakage, host-side configuration loading MUST skip hydration of plugin secrets. Secrets are resolved on-demand via the gRPC proxy, ensuring plaintext values never appear in the initial `config_json` handshake delivered to the plugin.
  - **Configurable Plugin Handshake:** To maintain seamless configuration fallbacks, plugins should implement the `sdk.Configurable` interface. The host's `config_json` is automatically delivered during the gRPC Handshake, allowing plugins to prioritize env-based secrets from the proxy while falling back to static config for non-sensitive fields (URLs, emails).
  - **Provider-Side Template Rendering:** Responsibility for rendering notes and templates belongs to the `Provider`. The CLI passes rich metadata (`NoteMetadata`) to the provider, allowing backends (especially plugins) to use specialized formats or logic without host awareness.
- **Linting:** Strict mode using `golangci-lint`.
- **Pre-commit Mandate:** NEVER push changes without running the full pre-commit suite, even if the automatic git hook fails to trigger. Local validation is the primary guardrail.

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
- **Implicit Default Bounds Trap:** Avoid "helpful" default time bounds (like `now - 30d`) in historical query paths if they aren't strictly required for performance. Providing a default lower bound when a user only specifies an upper bound (`--until`) creates a functional regression where old data is silently dropped. Always default to zero-value bounds (`time.Time{}`) for open-ended queries to match established CLI semantics.

### Development Strategy
- **SDK-First:** Prefer official Go SDKs (like Genkit for Google AI) over CLI wrappers for stability and robust streaming support.

### Persistence & State
- **Transient State vs. Immutable Audit Trail:** Separate high-churn execution state (`rig_orchestration`) from versioned observability events (`rig_events`). Transient state is for logic; versioned events are for auditing. This prevents performance degradation from constant Dolt commits on high-churn data.
- **Embedded Dolt Architecture:** For truly standalone, in-process versioned state (e.g., event tracking), use the `github.com/dolthub/driver` library. This allows Rig to act as its own SQL engine without requiring an external `dolt` binary or `sql-server` process. Access via the `dolt` driver name and `file://` DSN scheme.
- **Concurrency & Locking Strategy:**
  - **Row-Level Locking:** All state-modifying database methods use `SELECT ... FOR UPDATE` inside transactions to ensure atomic read-before-write operations.
  - **Retry with Backoff:** To handle Dolt serialization failures (MySQL Error 1213: deadlock, 1205: lock wait timeout) in multi-process scenarios, database write operations are wrapped in an exponential backoff retry loop (via `pkg/errors/retry.go`).
  - **Mandatory Transaction Restart:** **TRAP:** Simply retrying a failed SQL statement is insufficient. Each retry attempt MUST restart the entire transaction from `BeginTx` to ensure stale locks are cleared and the process sees the latest committed state.
  - **Versioning Scope:** `txAutoCommit` is only called for versioned data (Workflows, Definitions). Execution and Node state changes are transient and intentionally skip Dolt versioning.
- **Optional Telemetry Pattern:** Decouple telemetry (e.g., event logging) from core logic using an interface (e.g., `EventLogger`) and functional options (`WithEventLogger`). Always provide a `Noop` implementation as the default to ensure the system remains resilient if the telemetry store is unavailable or disabled.
- **Milestone Versioning Cadence:** To balance performance and auditability, log individual events via standard SQL `INSERT` and perform a `DOLT_COMMIT` only at significant milestones (e.g., workflow completion).
- **Linter-Resilient Query Building Pattern:** When constructing dynamic SQL, prefer `fmt.Sprintf` with a constant format string (e.g., `fmt.Sprintf("%s WHERE %s", base, conditions)`) over raw string concatenation. This satisfies `gosec` G202 (SQL injection) without requiring fragile `//nolint` directives that trigger `nolintlint` if the environment changes.
- **Safe Deprecation:** Use non-blocking configuration warnings (via `CheckSecurityWarnings`) instead of silent removal when deprecating keys to ensure a smooth user migration.
- **Bootstrap Package:** All early CLI orchestration (flag pre-parsing, configuration initialization, dynamic registration) belongs in `pkg/bootstrap` to keep `Execute()` paths delegative and testable.
- **Convention-Over-Collision:** Use uppercase shorthands for global host flags (e.g., `-C` for config) to minimize collisions with subcommand-specific shorthands.

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
- **Asymmetric IPC Timeouts Pattern:** Standardize on tiered timeouts for plugin RPCs. Fast operations (metadata, listing) use a 30s deadline, while heavy/network-bound operations (Clone, Worktree creation) use a 15m deadline to prevent functional regressions on slow networks.
- **Deferred Plugin Release Pattern:** Always call `defer Manager.ReleasePlugin(name)` immediately after successfully acquiring a plugin client. This ensures active session counts are correctly decremented even if subsequent logic fails, preventing "zombie" processes.

### Workflow Traps
- **Embedded Database Ambiguity:** Distinguish between "embedded data" (local files accessed via protocol) and "embedded engine" (in-process executor). Conflating the two can lead to incorrect driver selection and unnecessary dependency on external processes. Truly embedded logic uses library-backed drivers like `github.com/dolthub/driver`.
- **Ghost Event Ordering Trap:** Emitting observability events before authoritative state persistence creates a durable mismatch if the state update fails. **Mitigation:** Strictly follow the **State-First Persistence** pattern.
- **Dolt Driver JSON Scan Error**: The embedded Dolt driver returns JSON columns as `string`. Standard `Scan` into `json.RawMessage` (a `[]byte` slice) fails. **Mitigation**: Scan into a temporary `[]byte` variable first, then assign to the struct field.
- **Sparse-Checkout Staging:** In a `git sparse-checkout` environment, new files must be staged using `git add --sparse <path>` if they fall outside the current sparse index definition.
- **Surfacing Metadata Errors:** If a plugin's manifest file exists but is malformed, report an error rather than silently ignoring it. This prevents bypassing version checks due to configuration errors. Ensure consistent error reporting across all discovery paths (e.g., both single-binary and directory-based plugins).
- **Automated Reviewer Naming Conflicts**: Conflicting bot suggestions (e.g., snake_case vs. camelCase) should be resolved by prioritizing Go idioms and established codebase patterns over generic bot rules.
- **Stale Git Context in Hooks Trap**: Environment variables injected by `pre-commit` (e.g., `GIT_INDEX_FILE`) can interfere with unit tests that create temporary git repositories. Always unset `GIT_*` variables in test hooks.
- **Host Flag Shadowing**: In dynamic CLIs, bootstrap parsers MUST stop scanning at the first non-flag token or `--` to avoid "stealing" arguments from subcommands. Host-only flags must be strictly filtered before forwarding to sub-processes.
- **Circular Dependency Migration Trap:** Extracting existing logic (e.g., `pkg/git`) into abstractions (e.g., `pkg/vcs`) often triggers import cycles if shared types remain in the core. **Mitigation:** Extract shared DTOs, regexes, and utility functions into a dedicated "Leaf Package" (e.g., `pkg/vcs/url`) with zero dependencies on either side.
- **Package Shadowing in CLI Commands Trap:** Moving shared logic into a package with a common name (e.g., `pkg/ticket`) can lead to package shadowing if local variables in `cmd` use the same name (e.g., `ticket := "PROJ-123"`). This makes package-level functions inaccessible. **Mitigation:** Rename local CLI variables to be more specific (e.g., `ticketID`).
- **Empty TOML Unmarshal Nil Map Trap:** Programmatic TOML mutation (e.g., in `rig config set`) must account for the fact that unmarshalling comment-only or empty files can return a `nil` map. Always check and initialize the map before assignment to avoid panics.

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
- **Anti-Enumeration Secret Error Pattern:** The `SecretService` MUST return identical error responses (`codes.NotFound`) for both "missing" keys and "access denied" keys. This prevents malicious or buggy plugins from enumerating the host's secret keys. Distinguish between these states internally via structured errors (e.g. `ErrSecretNotFound`) for diagnostic clarity in host logs.
- **Stale Client Reference Trap:** Cached gRPC clients (e.g., `TicketClient`) in the `Plugin` struct MUST be explicitly nil-ed out in the `cleanup()` method during plugin stops or restarts. Failing to do so causes "connection closed" errors when the host attempts to use a stale client reference from a previous process execution.
- **Compatibility Translation Layer:** The host client should act as a translation shim, prioritizing modern structured fields (e.g., `plugin_semver`) but providing fallbacks/translation for legacy tags (e.g., `plugin_version`) to maintain wire compatibility during V1 migration.

#### Capability Modeling & AI
- **Assistant Capability:** Plugins advertising the `assistant` capability must implement the `AssistantService` (defined in `assistant.proto`) to provide AI completion and streaming services.
- **Discovery & Lifecycle:** The host's `PluginManager` discovers plugins via `Scanner`, starts them via `Executor`, and manages the connection state. Clients for specific capabilities (like `AssistantServiceClient`) are lazily initialized from the shared gRPC connection.
- **Dynamic Routing:** The AI factory (`NewProviderWithManager`) can route requests to specific assistant plugins by specifying the `plugin` provider and the plugin's name in the configuration.

#### UI Interaction Model
- **UIService (Host-as-Server)**: UI interaction is modeled as a callback service hosted by Rig. Plugins use the `UIService` (defined in `interaction.proto`) to request user input (Prompts, Confirms, Selects) or report progress.
- **Singleton Stdin Reader Pattern**: Centralize terminal input reading into a single background goroutine. Handlers send requests to this loop and wait for results or cancellation. This prevents goroutine leaks and input stealing when multiple plugins request input or when RPCs are canceled.
- **TUI Coordination**: A semaphore-based `Coordinator` ensures that only one blocking UI operation is active at a time, protecting terminal integrity.

### Protobuf & API Evolution
- **Tag Immortality:** Field tags in protobuf messages are permanent identities. Never reuse or repurpose a tag number for a different semantic meaning, even if renamed. Use `reserved` for removed tags to prevent future accidental reuse.
- **Hybrid Snake_Case Mapping:** Follow Go naming idioms for internal struct fields (e.g., `APIVersion` with all-caps initialisms) while using explicit struct tags (`json:"api_version"`) to satisfy project-wide `snake_case` requirements for external serialization.
- **API Standardisation:** Prefer industry-standard protocols (e.g., `grpc.health.v1`) over custom implementations for common infrastructure needs like health monitoring.
- **Fail-Fast Mocking:** When mocking complex gRPC interfaces, implementations MUST fail loudly (return error) for unset function fields rather than returning `nil`.
- **Stream Naming Inversion:** For bi-directional streams, name messages by *direction* (Request=Client->Server, Response=Server->Client) rather than payload semantics, but document the inversion clearly.

### Daemon & Process Management
- **Daemon-First Execution Pattern:** Always attempt execution via the background daemon first to minimize latency. Fall back to direct execution only on transport or availability failures (distinguished via `DaemonError`). Command execution failures (e.g., exit 1) should be returned directly, not retried.
- **Deferred Cleanup + Release Pattern:** When launching background processes, use a deferred cleanup function that kills and reaps the process unless a `released` flag is explicitly set upon success. This prevents zombie processes during failed handshakes or connection timeouts.
- **Bi-directional UI Proxying:** Use a single gRPC stream to multiplex command output and interactive UI requests. This ensures strict session isolation and eliminates the need for extra network ports or complex session routing.
- **Identity-Validated Signaling:** Before signaling a process (e.g., for shutdown), always verify its identity (via `CheckIdentity` helper or version RPC) to prevent accidental interference with reused PIDs.
- **Idempotent Resource Registration:** In asynchronous UI proxying, register response channels *before* sending requests to the client to eliminate registration-vs-response race conditions.
- **Single-Source Socket Truth:** Use `daemon.SocketPath()` as the sole source of truth for the daemon endpoint. Never hardcode or duplicate path logic in configuration if it can be derived from runtime context.
- **Atomic Idle Verification:** Re-verify plugin idle state (active sessions + last used time) while holding the manager lock before stopping a plugin to prevent races with new session acquisitions.

### UI & Stdin Management
- **Singleton Stdin Reader Pattern (v2)**: Stdin is a shared process resource. Manage it via a singleton `sharedReader` with a "No-Loss" buffer. If a UI request is abandoned, buffer the input for the next caller.
- **Sensitivity-Aware Buffering**: Never buffer sensitive input (passwords). Buffered responses must only be delivered if both the buffer and the current request are non-sensitive. Clear the buffer on sensitivity mismatch.
- **Buffered IO Synchronization**: Always drain/discard buffered bytes from `bufio.Reader` before performing raw terminal reads (e.g., `term.ReadPassword`) to prevent input reordering or "stolen" keystrokes.
- **Fail-Fast Shutdown Protocol**: UI request loops must monitor both the request context AND the global reader's shutdown signal (`done` channel) to prevent deadlocks during process exit.

### Plugin SDK Conventions
- **Type-Assertion Registration Pattern:** Use Go type assertions in the SDK's `Serve` or `Register` logic to automatically discover and register supported capability handlers (`AssistantHandler`, `CommandHandler`, `KnowledgeHandler`) from a base `PluginInfo` implementation.
- **Double-Resolver Addressing**: Use `passthrough://bufnet` (or similar) when using `bufconn` in tests to ensure cross-platform compatibility with gRPC resolvers (especially on Darwin).

### Resiliency Patterns
- **Lying State Resiliency**: Treat external state artifacts (like UDS socket files) as hints, not absolute truth. Respect the full timeout loop for daemon connection attempts even if a (potentially stale) socket file exists.
- **Daemon-CLI Scope Alignment**: Daemon plugin discovery must match CLI discovery scope. If a daemon cannot find a plugin (e.g., due to being started in a different directory), it MUST return a `codes.NotFound` gRPC error to trigger the CLI's fallback to direct execution.
- **Transparent Initialization Warnings:** Non-critical subsystem failures (like a misconfigured knowledge provider) should always surface a `Warning:` to the user rather than failing silently or crashing. This maintains UX transparency without blocking the primary workflow.

### Hierarchical Configuration System (V1)
- **5-Tier Precedence**: Settings resolve in strict order: **Flags > Env > Project Recursive > User > Defaults**.
- **CSS-Style Cascading**: Project configuration (`.rig.toml`) recursively merges from the Git root down to the current working directory.
- **LayeredLoader Orchestrator**: Centralize all configuration loading and merging in a `LayeredLoader` to manage provenance tracking and isolated test environments.
- **Provenance Tracking**: Maintain a `SourceMap` during the cascade to record exactly which tier (and file) provided each value, surfaced via `rig config inspect`.
- **Secure Hydration**: Use the `keychain://service/account` URI scheme for sensitive values to keep them out of plain-text configuration files while maintaining hierarchical precedence.
- **Format Strictness**: Strictly enforce TOML for all configuration files to ensure deep-merging behavior is predictable.
- **Rig Trust Model**: Protect the cascade with **Immutable Keys** (hardcoded sensitive keys like `github.token`) and a **Project Trust Gate**. Untrusted projects trigger warnings on override.
- **Fail-Closed Security Pattern**: In security-sensitive components (like the `TrustStore`), treat initialization failures or missing state as the most restrictive state (e.g., `untrusted`).
- **Atomic State Transitions**: Use the "temp-file + rename" pattern for local state persistence (e.g., `trusted-projects.json`) to prevent data corruption during writes.
- **Structured Discovery Trace**: Maintain an internal trace of resolution events (`DiscoveryEvent`) in the loader. This enables multi-format diagnostic output (Human/JSON) and unit testing of the discovery path without reliance on `stderr` parsing.

### Project Discovery Patterns
- **Unified Project Discovery**: Centralize project root detection and marker logic into `pkg/project`. Use a marker-based upward traversal engine that respects repository boundaries (`.git`, `.beads`).
- **Physical Path Truth**: Always resolve symlinks (`filepath.EvalSymlinks`) before traversal or caching to ensure a stable and unique project identity. This prevents "Symlink Mirage" cache misses.
- **Process-Level Caching**: Use a thread-safe singleton cache in `pkg/project` to memoize discovery results, eliminating redundant filesystem I/O for configuration and plugin lookups.

### Discovery & Configuration Traps
- **Implicit CWD Leakage**: Library-level discovery functions (like `Discover("")`) that default to CWD are risky for internal logic (e.g., ticket routing) as they can accidentally pick up the host's project context. Prefer explicit path inputs for internal components.
- **Hermetic Test Isolation**: Configuration and discovery tests MUST use `t.Chdir(t.TempDir())` or an explicit empty `projectCtx` to avoid reading the real `.rig.toml` from the repository or developer machine.
- **Global Cache Pollution**: Reset the discovery cache (`project.ResetCache()`) in tests that modify the working directory or simulate different project structures to ensure test isolation.
- **Linter Deprecation Formatting**: `golangci-lint` requires `Deprecated:` comments to be in their own paragraph, separated from the description by a blank line.
- **Secret Leakage in Warnings**: NEVER include the attempted value in security warning messages for immutable or sensitive keys. Printing the "blocked" value echoes potentially sensitive credentials to stderr/logs.
- **Ghost Trust Paths**: Trust entries must match discovered project roots. Rejection of non-project paths (paths without `.git` or `.rig.toml`) during `rig trust add` prevents users from trusting invalid paths that would never satisfy a trust check.
- **Sensitive Violation Leak**: Security violation logs (e.g., blocked overrides) often contain the `AttemptedValue`. Diagnostic outputs (especially JSON) must explicitly redact these values for sensitive keys, even if the primary configuration table is already redacted.
- **Config Cache Staleness**: Global configuration caches (e.g., `appConfig`) must be explicitly cleared via a reset hook (e.g., `resetConfig()`) in unit tests. Failing to do so causes "cross-test contamination" where stale configuration from one test leaks into another. Commands that require fresh data (like `debug`) should bypass or explicitly refresh this cache.

## Orchestration & Persistence (Dolt)

### Architectural Truths
- **Atomic Transactional Versioning**: Data changes and Dolt versioning (`CALL DOLT_COMMIT`) MUST be executed within the same SQL transaction to ensure consistency between the working set and version history.
- **Guard-Inside-Transaction Pattern**: Business invariants (e.g., active execution guards) MUST be checked inside a locked transaction (`SELECT ... FOR UPDATE`) to prevent race conditions under concurrent load.
- **Fetch-and-Merge Strategy**: To handle Go's zero-value ambiguity in update paths, fetch the current database state and merge it with the incoming payload before writing. This preserves existing status and ensures monotonic version increments.
- **Historical Entity Isolation**: Nodes and Edges are versioned alongside the Workflow definition. All queries for these entities MUST filter by `workflow_id` AND `workflow_version` to prevent "Version Bloat" and ensure historical execution integrity.

### Persistence Traps
- **Zero-Value Default Bypass**: Go's `database/sql` sends explicit zero-values (e.g., `""` for strings) to the database, which bypasses SQL `DEFAULT` clauses. Always normalize zero-valued fields in the DAL before insertion.
- **Strict ENUM Validation**: Dolt/MySQL ENUM columns reject empty strings if not explicitly part of the ENUM definition. Normalization is mandatory for status fields.
- **Integration Test Dependency**: Concurrency and locking tests are integration-tier and require a live Dolt environment via `RIG_TEST_DOLT_DSN`. They should be skipped in short mode.

### Plugin Security & Isolation
- **Back-Channel Resource Proxy Pattern**: Plugins are executed as isolated processes stripped of their environment. Privileged operations (Filesystem, Network) are proxied back to the host via an ephemeral gRPC-over-UDS service (`ResourceService`). The host explicitly grants capabilities (allowed paths, network access) per execution.
- **Parent-Recursive Symlink Defense**: Path validation must use recursive parent resolution (beyond `filepath.EvalSymlinks`) to prevent symlink escapes on non-existent files during write operations.
- **Resource Boundary Enforcement**: Always apply hard constraints (e.g., 10MB `io.LimitReader`) at the proxy layer for file and network IO to protect the host from resource exhaustion attacks.

### Configuration & Compatibility
- **Heuristic Envelope Detection Pattern**: Support both legacy flat JSON and modern wrapped (`plugin`/`capabilities`) configurations by prioritizing the explicit envelope. To prevent unintended privilege grants, only honor host-side capabilities if the explicit envelope is present.
- **Empty JSON Defaulting**: Always default empty or missing JSON configurations to a valid empty object (`{}`) rather than `nil` to prevent unmarshaling failures in downstream components.

### Orchestration & Recovery Patterns
- **Idempotent State Re-entry**: Allow transitions from a state to itself (e.g., `RUNNING -> RUNNING`) in the persistence layer to support crash recovery. Use SQL `COALESCE(started_at, ?)` to ensure the original start time is preserved during re-entry.
- **Node-Bridge Idempotency Contract**: All `NodeBridge` implementations MUST be idempotent. The orchestrator may re-invoke a node if it was interrupted mid-execution during a previous run.
- **Recovery Short-Circuit Pattern**: Detect pre-existing terminal failure states (including `SKIPPED` nodes) during recovery to prevent re-executing branches of a DAG that should have stayed dead.
- **Functional Dry-Run Validation Pattern**: Implement dry-run validation as a standalone, functional component (e.g., `DryRunValidate`) rather than a mode within the primary executor. Use environment-check interfaces (`PluginChecker`, `SecretResolver`) to keep the logic side-effect-free and portable. This enables rigorous static analysis of DAG structures, I/O schemas, and environment readiness without instantiating full orchestration dependencies.
- **Connection-Pinned Manual Transactions:** For operations requiring session state (e.g., `USE database` in migrations or sequential `DOLT_ADD/COMMIT`), acquire a single `*sql.Conn` and execute the entire sequence on that specific connection to avoid losing state in the pool.

### Persistence Traps (Orchestration)
- **Stale Memory Store State**: In-memory test stores (`MemoryStore`) must use strict `sync.Mutex` locking across all methods to prevent race conditions during concurrent workflow execution.
- **Key Construction Fragility**: Use `fmt.Sprintf("%s:%d", id, version)` for cache or map keys involving versions to ensure multi-digit support and prevent separator collisions.
- **Skipped Node Recovery Misclassification**: Recovery logic MUST treat pre-existing `SKIPPED` nodes as a failure indicator for the execution. If the process crashes after skipping nodes but before persisting the `FAILED` execution status, a recovery run might incorrectly declare `SUCCESS` if it only looks for explicit `FAILED` node rows.
- **Heuristic Wrapper Detection Trap**: When detecting "wrapped" vs "flat" JSON configurations, do not rely on the presence of a new metadata key (like `io`) as the sole signal. Heuristic detection must require an unambiguous signal (e.g., an explicit `plugin` key) or a strong quorum of reserved keys (e.g., `capabilities` + `io` with no other top-level keys). Failure to do so can lead to backward-incompatible changes where valid legacy configs are misidentified and their payloads are stripped.
- **Stacked Transaction Defers**: When executing a series of transactional operations (like migrations), extract the transaction logic into a helper function (e.g., `applyMigration`). This ensures `ROLLBACK` defers are executed immediately after each step rather than stacking up and potentially leaking resources or causing logic errors.

### Development & CI Traps
- **Linter Version Parity**: Discrepancies between local and CI `golangci-lint` versions can cause CI-only failures (like `prealloc`). Always align CI environment variables with the local development standard.
- **Go Zero-Value SQL Default Bypass**: Explicit zero-values in Go structs (e.g., `""`) bypass SQL `DEFAULT` clauses. Use `any(nil)` or `sql.Null*` types for optional columns (especially `JSON`) to trigger database defaults and avoid validation errors (e.g., `Invalid JSON text`).
- **Gosec G115 Panic (Go 1.26)**: The `gosec` G115 analyzer (integer overflow) triggers a catastrophic panic in `golangci-lint` when running on Go 1.26.0 with certain integer literals (e.g., `1000`). **Mitigation**: Exclude G115 at the analyzer level in `.golangci.yml` until `gosec` ≥ 2.24 is released.

## Architectural Patterns

### Metadata & Observability
- **State-First Persistence Pattern:** Always update the authoritative state store (e.g., orchestration DB) before emitting observability events or creating versioned history snapshots. This ensures the event history remains a truthful reflection of the system state.
- **Recursive Redaction Pattern:** Protect versioned history by implementing a centralized redaction layer that handles both unstructured strings (regex scrubbing) and structured JSON (recursive walking and key-based redaction) before data is persisted.
- **Milestone-Based Versioning Pattern:** In long-running workflows, trigger intermediate Dolt commits at significant milestones (e.g., node completion) to provide a "time-travel" audit trail and protect against data loss during process failure.
- **Decoupled Metadata Tagging**: Use narrow interfaces (e.g., `TicketMetadataSetter`) and retroactive backfilling to decorate data with context resolved after creation. This prevents import cycles and maintains business logic isolation from data storage.
- **Unified Presentation Model**: For CLI commands aggregating data from heterogeneous sources, use a flattened "Presentation Model" in a shared package. Map source-specific structs to this model using Go primitives to avoid circular dependencies and simplify sorting/formatting logic.
- **Deterministic Sort Ordering**: Always use a unique tie-breaker (e.g., `id ASC`) in SQL queries and `sort.SliceStable` in Go when rendering chronological lists. This prevents nondeterministic output churn in persistent artifacts like Markdown notes.
- **Hot-Path Core Isolation Decision:** Maintain high-frequency, low-latency filesystem predicates (like `IsGitRepo`) in the core `rig` binary rather than offloading them to plugins. This avoids IPC overhead for "snappy" CLI discovery while still decoupling semantic VCS logic into plugins.

### Maintenance & Storage
- **Maintenance Connection Pinning:** Maintenance operations in Dolt (e.g., `USE database`, `dolt_gc`, multi-table pruning) MUST use a pinned `*sql.Conn` to ensure session affinity. Using the general pool can lead to commands running against the wrong database context.
- **Authoritative Pruning Feedback:** Rely on `RowsAffected()` from the database after a `DELETE` for reporting and logs, rather than pre-calculating counts. This avoids race conditions and ensures reporting is a truthful reflection of the disk state. In dry-run modes, use an explicit `SELECT COUNT(*)` with matching filters.

### Storage & Retention Traps
- **Minimum Retention Fallacy:** When managing multiple heterogeneous stores (e.g., events and orchestration), never derive a single "global" cutoff based on the minimum retention. Each store must calculate its own cutoff independently to avoid premature data loss in stores with longer retention guarantees.
