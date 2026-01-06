# Release Notes: v0.1.0

## Overview

First release of the SRE CLI tool—a Go-based workflow automation utility that integrates git worktrees, tmux sessions, Obsidian notes, JIRA, and command history tracking into a unified developer experience.

Version 0.1.0 signals a usable, production-ready foundation with sound architecture. The API surface may evolve before v1.0.0.

## Installation

### Homebrew (recommended)

```bash
brew install thoreinstein/tap/sre
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/sre/releases/tag/v0.1.0)
2. Extract and move to your PATH:

```bash
tar -xzf sre_0.1.0_darwin_arm64.tar.gz
mv sre /usr/local/bin/
```

3. Verify installation:

```bash
sre version
```

## Features

### Core Workflow Commands

- **`sre init <ticket>`** — Initialize a new ticket workflow with git worktree, tmux session, and Obsidian notes
- **`sre hack <name>`** — Start lightweight non-ticket workflows for experiments and quick tasks
- **`sre list`** — Display active worktrees and tmux sessions
- **`sre clean`** — Remove old worktrees and clean up stale sessions

### Session Management

- **`sre session list`** — List all tmux sessions
- **`sre session attach <name>`** — Attach to an existing session
- **`sre session kill <name>`** — Terminate a session

### History and Timeline

- **`sre timeline`** — View chronological activity across workflows
- **`sre history query <pattern>`** — Search command history
- **`sre history info`** — Display history database statistics

### Configuration

- **`sre config`** — Display current configuration
- **`sre config --show`** — Show resolved configuration with defaults

### Synchronization

- **`sre sync`** — Synchronize state across tools (git, notes, sessions)

### Self-Update

- **`sre update`** — Update to the latest release with checksum validation
  - `--check` — Check for updates without installing
  - `--force` — Force update even if current version matches
  - `--yes` — Skip confirmation prompt

### Multi-Repository Support

Route tickets to different repositories based on ticket type prefixes:

```toml
[ticket_types]
  [ticket_types.INFRA]
  repo = "~/src/infrastructure"

  [ticket_types.APP]
  repo = "~/src/application"
```

## Breaking Changes

### Config Format: YAML → TOML

Configuration has migrated from YAML to TOML. Manual migration required.

**Before (`~/.config/sre/config.yaml`):**

```yaml
jira:
  url: https://company.atlassian.net
  project: SRE
obsidian:
  vault: ~/notes
tmux:
  windows:
    - name: editor
      command: nvim
    - name: shell
```

**After (`~/.config/sre/config.toml`):**

```toml
[jira]
url = "https://company.atlassian.net"
project = "SRE"

[obsidian]
vault = "~/notes"

[[tmux.windows]]
name = "editor"
command = "nvim"

[[tmux.windows]]
name = "shell"
```

### JIRA Client API

The JIRA client now returns errors explicitly. Update any code that calls the client directly:

```go
// Before
ticket := client.GetTicket(id)

// After
ticket, err := client.GetTicket(id)
if err != nil {
    return fmt.Errorf("fetch ticket: %w", err)
}
```

### File Permissions

File and directory permissions are now more restrictive:
- Files: `0600` (owner read/write only)
- Directories: `0700` (owner read/write/execute only)

Scripts or tools that expect group/world-readable permissions will need adjustment.

## Security

### Input Validation

- **Command injection prevention** — Hack names validated against strict regex pattern
- **Path traversal protection** — All file paths sanitized before use
- **Tmux command allowlist** — Only permitted tmux operations execute

### Supply Chain Security

- **Sigstore signing** — All release artifacts signed with keyless cosign
- **SBOM generation** — Software Bill of Materials included for transparency
- **SHA-pinned Actions** — All GitHub Actions pinned to specific commit SHAs
- **Least-privilege workflows** — CI/CD runs with minimal required permissions

### File Permissions Hardening

All created files use restrictive permissions by default:
- Configuration files: `0600`
- Data directories: `0700`
- Database files: `0600`

## Bug Fixes

- **Race conditions in tests** — Removed `t.Parallel()` from tests with shared state
- **Tmux windows not created** — Fixed type mismatch in viper defaults; `[]TmuxWindow` now correctly parsed instead of `[]map[string]interface{}`
- **CGO dependency removed** — Migrated from `mattn/go-sqlite3` to `modernc.org/sqlite` for pure Go builds (`CGO_ENABLED=0`)

## Known Limitations

Items to address before v1.0.0:

- **No automated config migration** — Users must manually convert YAML to TOML
- **Pre-release version detection** — Update command does not yet detect or offer pre-release versions
- **Limited error recovery** — Some edge cases (network failures, partial state) have minimal recovery logic

## Migration Guide

### Step 1: Backup Existing Configuration

```bash
cp ~/.config/sre/config.yaml ~/.config/sre/config.yaml.bak
```

### Step 2: Convert YAML to TOML

Create `~/.config/sre/config.toml` with equivalent TOML syntax. Key differences:

| YAML | TOML |
|------|------|
| `key: value` | `key = "value"` |
| Nested maps use indentation | Nested maps use `[section]` headers |
| Lists use `- item` | Arrays of tables use `[[section]]` |

### Step 3: Update Scripts

If you have scripts calling the JIRA client programmatically, update error handling as shown in Breaking Changes above.

### Step 4: Verify Configuration

```bash
sre config --show
```

Confirm all settings resolved correctly. Remove the old YAML file once verified:

```bash
rm ~/.config/sre/config.yaml
```

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Install cosign if needed
brew install cosign

# Download checksums and signature
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.1.0/checksums.txt
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.1.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.1.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/sre/.github/workflows/release.yml@refs/tags/v0.1.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

Successful verification confirms the checksums file was signed by the official GitHub Actions release workflow and your downloaded artifact matches the expected hash.
