# Release Notes: v0.6.0

## Overview

This release introduces direct Jira REST API integration, enabling native communication with Jira Cloud without requiring external CLI tools. The new API client supports rate limiting, automatic retries, and custom field display while maintaining full backward compatibility with existing CLI-based configurations.

**Release date:** 2026-01-12

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/rig
# or for fresh install:
brew install thoreinstein/tap/rig
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/rig/releases/tag/v0.6.0)
2. Extract and move to your PATH:

```bash
tar -xzf rig_0.6.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

3. Verify installation:

```bash
rig version
```

## Features

### Direct Jira REST API Integration

The new APIClient provides native Jira Cloud REST API v3 support without requiring external CLI dependencies:

- **Authentication**: Basic auth via email and API token (supports `JIRA_TOKEN` environment variable)
- **Rate Limiting**: Exponential backoff with configurable retries (max 3 retries, ±20% jitter)
- **Retry-After Support**: Automatically respects Jira's rate limit headers
- **ADF Parsing**: Native Atlassian Document Format parsing for issue descriptions

### Dual-Mode Architecture

A factory pattern allows seamless selection between implementations:

| Mode       | Implementation     | Use Case                                |
| ---------- | ------------------ | --------------------------------------- |
| `api`        | New APIClient      | Direct REST API access, no CLI required |
| `acli`       | Existing CLIClient | Legacy Atlassian CLI integration        |
| `""` (empty) | CLIClient          | Backward compatible default             |

### Custom Fields Support

Display custom Jira fields in your notes with configurable field mappings:

- Map friendly names to Jira field IDs
- Supports strings, numbers, objects, and arrays
- Configure via `jira.custom_fields` in your config file

## Configuration

### New Configuration Options

```yaml
jira:
  enabled: true
  mode: "api"                                     # "api" or "acli" (default: acli)
  base_url: "https://your-domain.atlassian.net"   # Required for API mode
  email: "user@example.com"                       # Required for API mode
  token: ""                                       # Or use JIRA_TOKEN env var
  custom_fields:                                  # Map friendly names to field IDs
    story_points: "customfield_10016"
    team: "customfield_10001"
```

### Environment Variables

| Variable   | Description                                 |
| ---------- | ------------------------------------------- |
| `JIRA_TOKEN` | Jira API token (alternative to config file) |

### Backward Compatibility

Existing configurations continue to work unchanged:

- Empty or unset `mode` defaults to `acli`
- All existing CLI-based configurations are fully supported
- No migration required for current users

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.6.0/checksums.txt
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.6.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.6.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/rig/.github/workflows/release.yml@refs/tags/v0.6.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.5.0:

```bash
# Homebrew
brew uninstall rig
brew install thoreinstein/tap/rig@0.5.0

# Manual
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.5.0/rig_0.5.0_darwin_arm64.tar.gz
tar -xzf rig_0.5.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

# Release Notes: v0.5.0

## Overview

The "Identity Release" completes the rename from `sre` to `rig`. This release updates the Go module path, binary name, and configuration directory to use the `rig` identity throughout. The name captures what the tool does: it **rigs up** your development environment—wiring together git worktrees, tmux sessions, and documentation into a cohesive workflow.

**Release date:** 2026-01-08

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/rig
# or for fresh install:
brew install thoreinstein/tap/rig
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/rig/releases/tag/v0.5.0)
2. Extract and move to your PATH:

```bash
tar -xzf rig_0.5.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

3. Verify installation:

```bash
rig version
```

## Breaking Changes

### Go Module Path Renamed

The Go module has been renamed from `thoreinstein.com/sre` to `thoreinstein.com/rig`.

| Before (v0.4.x)      | After (v0.5.0)       |
| -------------------- | -------------------- |
| `thoreinstein.com/sre` | `thoreinstein.com/rig` |

**Impact:** Users who import this module in Go code must update their import paths.

### Binary Name Changed

The CLI binary has been renamed from `sre` to `rig`.

| Before (v0.4.x) | After (v0.5.0) |
| --------------- | -------------- |
| `sre`             | `rig`            |

**Impact:** Scripts, shell aliases, and PATH references that invoke `sre` will break.

### Configuration Directory Moved

The configuration directory has moved to match the new identity.

| Before (v0.4.x) | After (v0.5.0) |
| --------------- | -------------- |
| `~/.config/sre/`  | `~/.config/rig/` |

**Impact:** Existing configuration files must be migrated to the new location.

**Note:** Environment variables already use the `RIG_*` prefix (changed in v0.4.0), so no environment variable changes are required for this release.

## Migration Guide

### Step 1: Move Configuration Directory

```bash
# Move config to new location
mv ~/.config/sre ~/.config/rig

# Verify configuration is recognized
rig config list
```

### Step 2: Update Shell Aliases

Replace references to `sre` with `rig` in your shell configuration:

```bash
# Check for affected aliases
grep -E '\bsre\b' ~/.bashrc ~/.zshrc ~/.config/fish/config.fish 2>/dev/null

# Example: Update alias
# Before:
alias sw="sre work"

# After:
alias sw="rig work"
```

### Step 3: Update Scripts

Replace `sre` with `rig` in any automation scripts:

```bash
# Find scripts referencing sre
grep -r '\bsre\b' ~/bin ~/.local/bin /usr/local/bin 2>/dev/null

# Update references
sed -i 's/\bsre\b/rig/g' ~/bin/my-workflow.sh
```

### Step 4: Update Go Import Paths (if applicable)

If you import this module in Go code:

```go
// Before
import "thoreinstein.com/sre/pkg/git"

// After
import "thoreinstein.com/rig/pkg/git"
```

## Other Changes

- **Code style improvements** — Cosmetic lint fixes across 13 command files for consistent formatting

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.5.0/checksums.txt
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.5.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.5.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/rig/.github/workflows/release.yml@refs/tags/v0.5.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.4.1:

```bash
# Homebrew
brew uninstall rig
brew install thoreinstein/tap/rig@0.4.1

# Manual
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.1/rig_0.4.1_darwin_arm64.tar.gz
tar -xzf rig_0.4.1_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

**After rollback:** Move your configuration back to the old location:

```bash
mv ~/.config/rig ~/.config/sre
```

Revert any alias or script changes (`rig` → `sre`) if you updated them for v0.5.0.

# Release Notes: v0.4.1

## Overview

This patch release fixes tmux window creation on systems using the default `base-index=0` setting. Previously, window indices were hardcoded to start at 1, causing session creation failures when tmux's default 0-based indexing was in effect. The fix queries and respects the user's configured `base-index` value.

**Release date:** 2026-01-08

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/rig
# or for fresh install:
brew install thoreinstein/tap/rig
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/rig/releases/tag/v0.4.1)
2. Extract and move to your PATH:

```bash
tar -xzf rig_0.4.1_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

3. Verify installation:

```bash
rig version
```

## Bug Fixes

### Fixed: Tmux Window Indexing Ignores `base-index` Setting

**Symptom:** Running `rig work` or `rig hack` failed to create tmux sessions on systems using the default `base-index=0` configuration.

**Root Cause:** Window indices were hardcoded to start at 1, but tmux defaults to 0-based indexing. Users who hadn't set `base-index=1` in their tmux configuration experienced session creation failures.

**Fix:** Added `getBaseIndex()` method that queries `tmux show-options -g base-index` to respect the user's configured value. The result is cached with `sync.Once` (queried once per SessionManager lifetime) and falls back to 0 (tmux default) on any failure.

**Impact:** Transparent improvement—existing configurations continue to work, and systems using the tmux default now work correctly.

## CI/Test Improvements

- Added tmux installation to CI workflow to enable tmux integration tests

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.1/checksums.txt
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.1/checksums.txt.sig
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.1/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/rig/.github/workflows/release.yml@refs/tags/v0.4.1' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.4.0:

```bash
# Homebrew
brew uninstall rig
brew install thoreinstein/tap/rig@0.4.0

# Manual
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.0/rig_0.4.0_darwin_arm64.tar.gz
tar -xzf rig_0.4.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

**Note:** After rollback, tmux session creation will fail on systems using `base-index=0`. As a workaround, add `set-option -g base-index 1` to your `~/.tmux.conf`.

# Release Notes: v0.4.0

## Overview

This release fixes environment variable pollution that corrupted tmux configuration when running the CLI inside an existing tmux session. All environment variable bindings now require an `RIG_` prefix, which is a **breaking change** for users who set configuration via environment variables.

**Release date:** 2026-01-07

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/rig
# or for fresh install:
brew install thoreinstein/tap/rig
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/rig/releases/tag/v0.4.0)
2. Extract and move to your PATH:

```bash
tar -xzf rig_0.4.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

3. Verify installation:

```bash
rig version
```

## Breaking Changes

### Environment Variables Require `RIG_` Prefix

Environment variables used to configure the CLI must now include an `RIG_` prefix.

**Rationale:** The tmux program sets a `TMUX` environment variable (e.g., `/private/tmp/tmux-502/default,12345,0`) in all processes it spawns. Viper's automatic environment binding was mapping this to the `tmux` config key, overwriting tmux window configuration and causing unpredictable behavior when running `rig` commands inside a tmux session.

Adding `SetEnvPrefix("SRE")` ensures only explicitly-intended environment variables affect configuration.

**Migration Table:**

| Before (v0.3.x) | After (v0.4.0)      |
| --------------- | ------------------- |
| `NOTES_PATH`      | `RIG_NOTES_PATH`      |
| `CLONE_BASE_PATH` | `RIG_CLONE_BASE_PATH` |
| `VERBOSE`         | `RIG_VERBOSE`         |

**Nested Configuration:**

For nested config keys, use underscore separators after the `RIG_` prefix:

| Config Key          | Environment Variable    |
| ------------------- | ----------------------- |
| `notes.path`          | `RIG_NOTES_PATH`          |
| `clone.base_path`     | `RIG_CLONE_BASE_PATH`     |
| `tmux.session_prefix` | `RIG_TMUX_SESSION_PREFIX` |
| `tmux.windows`        | `RIG_TMUX_WINDOWS`        |

**Impact:** Shell profiles, CI/CD pipelines, and container configurations that set these environment variables will stop working until updated.

**Migration Script:**

```bash
# Check for affected environment variables in shell configs
grep -E '^\s*export\s+(NOTES_PATH|CLONE_BASE_PATH|VERBOSE)=' \
  ~/.bashrc ~/.zshrc ~/.profile ~/.bash_profile 2>/dev/null

# Example fix in .zshrc
# Before:
export NOTES_PATH=~/notes/work

# After:
export RIG_NOTES_PATH=~/notes/work
```

## Bug Fixes

### Fixed: Tmux Configuration Corruption in Nested Sessions

**Symptom:** Running `rig work` or `rig hack` inside an existing tmux session would fail to create configured windows, or create sessions with wrong window layouts.

**Root Cause:** The `TMUX` environment variable (set by tmux itself) was being bound to Viper's `tmux` config key, overwriting the user's tmux window configuration with the socket path string.

**Fix:** Environment variables now require the `RIG_` prefix, preventing pollution from unrelated environment variables like `TMUX`, `TERM`, `PATH`, etc.

## Test Improvements

- **Clone URL parsing tests** now call `git.ParseGitHubURL` directly instead of `runCloneCommand`, eliminating network dependencies and improving reliability
- **New integration test** `TestCreateSession_CreatesAllWindows` verifies tmux sessions are created with all configured windows, catching window creation regressions

## Developer Experience

- **Removed `go-test` hook** from pre-commit configuration — tests no longer run on every commit, significantly speeding up local development iteration
- Pre-commit still runs `go-vet` and `go-build` to catch obvious issues
- Full test suite runs in CI before merge

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.0/checksums.txt
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.4.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/rig/.github/workflows/release.yml@refs/tags/v0.4.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.3.0:

```bash
# Homebrew
brew uninstall rig
brew install thoreinstein/tap/rig@0.3.0

# Manual
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.3.0/rig_0.3.0_darwin_arm64.tar.gz
tar -xzf rig_0.3.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

**After rollback:** Revert environment variable changes (`RIG_NOTES_PATH` → `NOTES_PATH`) if you updated them for v0.4.0. Note that the tmux configuration corruption bug will return if you run `rig` commands inside tmux sessions.

# Release Notes: v0.3.0

## Overview

This release introduces the `rig clone` command, enabling structured repository management with automatic worktree workflow support. Repositories are cloned to a consistent `~/src/<owner>/<repo>` layout, with SSH URLs automatically configured for bare clone + worktree workflows optimized for multi-branch development.

**Release date:** 2026-01-06

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/rig
# or for fresh install:
brew install thoreinstein/tap/rig
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/rig/releases/tag/v0.3.0)
2. Extract and move to your PATH:

```bash
tar -xzf rig_0.3.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

3. Verify installation:

```bash
rig version
```

## Features

### New Command: `rig clone`

Clone GitHub repositories into a structured directory layout that integrates seamlessly with `rig hack` and `rig work` commands.

**Usage:**

```bash
# SSH URL — creates bare repository with worktree workflow
rig clone git@github.com:owner/repo.git

# HTTPS URL — standard git clone
rig clone https://github.com/owner/repo.git

# Shorthand format
rig clone github.com/owner/repo
```

**Directory Structure:**

All repositories are cloned to `~/src/<owner>/<repo>`:

```
~/src/
├── thoreinstein/
│   └── rig/           # Bare repo (SSH) or standard clone (HTTPS)
├── golang/
│   └── go/
└── kubernetes/
    └── kubernetes/
```

**SSH vs HTTPS Behavior:**

| URL Type                       | Clone Method   | Workflow                                         |
| ------------------------------ | -------------- | ------------------------------------------------ |
| SSH (`git@github.com:...`)       | Bare clone     | Worktree-based development via `rig hack`/`rig work` |
| HTTPS (`https://github.com/...`) | Standard clone | Traditional branch-based development             |

**Configuration:**

Customize the base path via configuration:

```bash
# Set custom base path
rig config set clone.base_path ~/code

# View current setting
rig config get clone.base_path
```

**Key Behaviors:**

- **Idempotent** — Existing repositories are detected and skipped
- **SSH optimization** — Bare clone + worktree workflow ready for multi-branch development
- **Natural integration** — Cloned repos work immediately with `rig hack` and `rig work`

### Example Workflow

```bash
# Clone a repository
rig clone git@github.com:thoreinstein/rig.git

# Start work on a ticket (creates worktree)
cd ~/src/thoreinstein/rig
rig work PROJ-1234

# Or start a hack session
rig hack feature-branch
```

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.3.0/checksums.txt
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.3.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.3.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/rig/.github/workflows/release.yml@refs/tags/v0.3.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.2.0:

```bash
# Homebrew
brew uninstall rig
brew install thoreinstein/tap/rig@0.2.0

# Manual
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.2.0/rig_0.2.0_darwin_arm64.tar.gz
tar -xzf rig_0.2.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

# Release Notes: v0.2.0

## Overview

This release introduces a breaking CLI change and adds automatic repair for bare repository configurations. The `init` command has been renamed to `work` to better reflect its purpose—starting work on a ticket, not initializing infrastructure.

**Release date:** 2026-01-06

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/rig
# or for fresh install:
brew install thoreinstein/tap/rig
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/rig/releases/tag/v0.2.0)
2. Extract and move to your PATH:

```bash
tar -xzf rig_0.2.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

3. Verify installation:

```bash
rig version
```

## Breaking Changes

### CLI Rename: `init` → `rig work`

The `init` command has been renamed to `work`.

**Rationale:** "init" implies initialization or setup, but this command starts work on a ticket—creating a worktree, tmux session, and notes. "work" accurately describes the intent.

**Before:**

```bash
init PROJ-1234
```

**After:**

```bash
rig work PROJ-1234
```

**Impact:** Scripts, shell aliases, and muscle memory that reference `init` will break.

## Features

### Auto-Repair for Bare Repository Fetch Refspec

Bare repositories created with `git clone --bare` lack the fetch refspec needed for remote tracking. Previously, users had to manually configure this:

```bash
git config remote.origin.fetch "+refs/heads/*:refs/remotes/origin/*"
```

The tool now detects missing fetch refspecs and adds them automatically. This repair is:

- **Idempotent** — Safe to run repeatedly; existing refspecs are preserved
- **Non-fatal** — Warns on failure and continues execution
- **Transparent** — No user action required

### Test Infrastructure Improvements

Internal changes to improve test reliability:

- Tests now run on an isolated tmux socket (`RIG_TEST_TMUX_SOCKET` environment variable)
- `TestMain` pattern ensures cleanup of test sessions even on failures
- Zero risk of test artifacts appearing in user workspace

## Bug Fixes

- Fixed test cleanup by adding `TestMain` to terminate tmux sessions after test runs

## Dependencies

- `modernc.org/sqlite`: 1.41.0 → 1.42.2

## Migration Guide

### Step 1: Update Scripts and Aliases

Replace all occurrences of `init` with `rig work`:

```bash
# One-liner for scripts
sed -i 's/init/rig work/g' ~/.local/bin/my-workflow.sh

# Check shell config files
grep -r "init" ~/.bashrc ~/.zshrc ~/.config/fish/
```

### Step 2: Update Shell Aliases

If you have aliases defined:

```bash
# Before
alias si="init"

# After
alias sw="rig work"
```

### Step 3: Rebuild Muscle Memory

The command is now `rig work <ticket>`. Tab completion (if configured) will reflect the new command name after upgrade.

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.2.0/checksums.txt
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.2.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.2.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/rig/.github/workflows/release.yml@refs/tags/v0.2.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.1.0:

```bash
# Homebrew
brew uninstall rig
brew install thoreinstein/tap/rig@0.1.0

# Manual
curl -LO https://github.com/thoreinstein/rig/releases/download/v0.1.0/rig_0.1.0_darwin_arm64.tar.gz
tar -xzf rig_0.1.0_darwin_arm64.tar.gz
mv rig /usr/local/bin/
```

Remember to revert any script changes (`rig work` → `init`) if rolling back.
