# Release Notes: v0.3.0

## Overview

This release introduces the `sre clone` command, enabling structured repository management with automatic worktree workflow support. Repositories are cloned to a consistent `~/src/<owner>/<repo>` layout, with SSH URLs automatically configured for bare clone + worktree workflows optimized for multi-branch development.

**Release date:** 2026-01-06

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/sre
# or for fresh install:
brew install thoreinstein/tap/sre
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/sre/releases/tag/v0.3.0)
2. Extract and move to your PATH:

```bash
tar -xzf sre_0.3.0_darwin_arm64.tar.gz
mv sre /usr/local/bin/
```

3. Verify installation:

```bash
sre version
```

## Features

### New Command: `sre clone`

Clone GitHub repositories into a structured directory layout that integrates seamlessly with `sre hack` and `sre work` commands.

**Usage:**

```bash
# SSH URL — creates bare repository with worktree workflow
sre clone git@github.com:owner/repo.git

# HTTPS URL — standard git clone
sre clone https://github.com/owner/repo.git

# Shorthand format
sre clone github.com/owner/repo
```

**Directory Structure:**

All repositories are cloned to `~/src/<owner>/<repo>`:

```
~/src/
├── thoreinstein/
│   └── sre/           # Bare repo (SSH) or standard clone (HTTPS)
├── golang/
│   └── go/
└── kubernetes/
    └── kubernetes/
```

**SSH vs HTTPS Behavior:**

| URL Type                       | Clone Method   | Workflow                                         |
| ------------------------------ | -------------- | ------------------------------------------------ |
| SSH (`git@github.com:...`)       | Bare clone     | Worktree-based development via `sre hack`/`sre work` |
| HTTPS (`https://github.com/...`) | Standard clone | Traditional branch-based development             |

**Configuration:**

Customize the base path via configuration:

```bash
# Set custom base path
sre config set clone.base_path ~/code

# View current setting
sre config get clone.base_path
```

**Key Behaviors:**

- **Idempotent** — Existing repositories are detected and skipped
- **SSH optimization** — Bare clone + worktree workflow ready for multi-branch development
- **Natural integration** — Cloned repos work immediately with `sre hack` and `sre work`

### Example Workflow

```bash
# Clone a repository
sre clone git@github.com:thoreinstein/sre.git

# Start work on a ticket (creates worktree)
cd ~/src/thoreinstein/sre
sre work PROJ-1234

# Or start a hack session
sre hack feature-branch
```

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.3.0/checksums.txt
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.3.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.3.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/sre/.github/workflows/release.yml@refs/tags/v0.3.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.2.0:

```bash
# Homebrew
brew uninstall sre
brew install thoreinstein/tap/sre@0.2.0

# Manual
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.2.0/sre_0.2.0_darwin_arm64.tar.gz
tar -xzf sre_0.2.0_darwin_arm64.tar.gz
mv sre /usr/local/bin/
```

# Release Notes: v0.2.0

## Overview

This release introduces a breaking CLI change and adds automatic repair for bare repository configurations. The `sre init` command has been renamed to `sre work` to better reflect its purpose—starting work on a ticket, not initializing infrastructure.

**Release date:** 2026-01-06

## Installation

### Homebrew (recommended)

```bash
brew upgrade thoreinstein/tap/sre
# or for fresh install:
brew install thoreinstein/tap/sre
```

### Manual Installation

1. Download the appropriate archive from the [releases page](https://github.com/thoreinstein/sre/releases/tag/v0.2.0)
2. Extract and move to your PATH:

```bash
tar -xzf sre_0.2.0_darwin_arm64.tar.gz
mv sre /usr/local/bin/
```

3. Verify installation:

```bash
sre version
```

## Breaking Changes

### CLI Rename: `sre init` → `sre work`

The `sre init` command has been renamed to `sre work`.

**Rationale:** "init" implies initialization or setup, but this command starts work on a ticket—creating a worktree, tmux session, and notes. "work" accurately describes the intent.

**Before:**

```bash
sre init PROJ-1234
```

**After:**

```bash
sre work PROJ-1234
```

**Impact:** Scripts, shell aliases, and muscle memory that reference `sre init` will break.

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

- Tests now run on an isolated tmux socket (`SRE_TEST_TMUX_SOCKET` environment variable)
- `TestMain` pattern ensures cleanup of test sessions even on failures
- Zero risk of test artifacts appearing in user workspace

## Bug Fixes

- Fixed test cleanup by adding `TestMain` to terminate tmux sessions after test runs

## Dependencies

- `modernc.org/sqlite`: 1.41.0 → 1.42.2

## Migration Guide

### Step 1: Update Scripts and Aliases

Replace all occurrences of `sre init` with `sre work`:

```bash
# One-liner for scripts
sed -i 's/sre init/sre work/g' ~/.local/bin/my-workflow.sh

# Check shell config files
grep -r "sre init" ~/.bashrc ~/.zshrc ~/.config/fish/
```

### Step 2: Update Shell Aliases

If you have aliases defined:

```bash
# Before
alias si="sre init"

# After
alias sw="sre work"
```

### Step 3: Rebuild Muscle Memory

The command is now `sre work <ticket>`. Tab completion (if configured) will reflect the new command name after upgrade.

## Verification

All releases are signed with [keyless Sigstore](https://www.sigstore.dev/). Verify the checksums file signature:

```bash
# Download checksums and signature
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.2.0/checksums.txt
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.2.0/checksums.txt.sig
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.2.0/checksums.txt.bundle

# Verify signature
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/thoreinstein/sre/.github/workflows/release.yml@refs/tags/v0.2.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your download against checksums
sha256sum --check checksums.txt --ignore-missing
```

## Rollback

If you need to revert to v0.1.0:

```bash
# Homebrew
brew uninstall sre
brew install thoreinstein/tap/sre@0.1.0

# Manual
curl -LO https://github.com/thoreinstein/sre/releases/download/v0.1.0/sre_0.1.0_darwin_arm64.tar.gz
tar -xzf sre_0.1.0_darwin_arm64.tar.gz
mv sre /usr/local/bin/
```

Remember to revert any script changes (`sre work` → `sre init`) if rolling back.
