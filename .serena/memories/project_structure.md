# Project Structure

## Root
- `main.go`: Entry point.
- `GEMINI.md`: AI Context & Architectural Decisions.
- `project.yaml`: Project metadata.

## Directories
- `cmd/`: Cobra CLI command implementations.
- `pkg/`: Core business logic libraries.
    - `pkg/ai/`: AI Provider integrations (Gemini, Anthropic).
    - `pkg/api/v1/`: gRPC Protobuf definitions (V1 Contract).
    - `pkg/config/`: Configuration handling (Viper).
    - `pkg/git/`: Git worktree and repo management.
    - `pkg/workflow/`: Workflow engine (V1 dynamic engine).
    - `pkg/plugin/`: Plugin loader and runner.
    - `pkg/beads/`: Beads issue tracker integration.
    - `pkg/history/`: SQLite/Dolt history tracking.

## V1 Architecture (In Progress)
- **Core**: Moving to a micro-kernel architecture.
- **Plugins**: Decoupling logic into gRPC plugins.
- **Config**: Implementing cascading config (`.rig.toml`).
- **Data**: Migrating internal state to Dolt.
