# Rig

A modern, extensible Go CLI tool for developer workflow automation. This tool replaces a complex bash script with a maintainable, feature-rich application that integrates with Git worktrees, Tmux sessions, Markdown documentation, and command history tracking.

## Features

### 🚀 **Complete Workflow Automation**

- **Ticket-based workflows**: Initialize complete work environments with a single command
- **Git worktree integration**: Automatic isolated workspaces per ticket
- **Tmux session management**: Configurable multi-window terminal sessions
- **Markdown note creation**: Template-based documentation with JIRA integration
- **Command history tracking**: SQLite-based timeline export and analysis

### 🛠️ **Powerful CLI Interface**

```bash
rig work <ticket>              # Start complete workflow
rig hack <name>                # Lightweight workflow for non-ticket work
rig list                       # Show all worktrees and tmux sessions
rig clean                      # Remove old worktrees and sessions
rig session list/attach/kill   # Manage tmux sessions
rig timeline <ticket>          # Export command history timeline
rig history query [pattern]    # Query command database
rig sync <ticket>              # Update notes and JIRA info
rig config --show/--init       # Manage configuration
```

### 🔧 **Integrations**

- **Git**: Automatic worktree and branch creation with base branch detection
- **JIRA**: Ticket metadata fetching via configurable CLI tools (`acli`)
- **Markdown**: Rich note templates, daily note updates, timeline export
- **Tmux**: Session automation with environment variables and window layouts
- **History Databases**: Support for both zsh-histdb and atuin SQLite schemas

## Quick Start

### Installation

1. **Clone the repository**:

   ```bash
   git clone <repository-url>
   cd rig
   ```

2. **Build the CLI**:

   ```bash
   cd main/
   go build -o rig
   ```

3. **Initialize configuration**:

   ```bash
   ./rig config --init
   ```

4. **Edit configuration** at `~/.config/rig/config.yaml`:
   ```yaml
   vault:
     path: "~/Documents/Second Brain"
   repository:
     owner: "your-org"
     name: "your-repo"
     base_path: "~/src"
   ```

### Basic Usage

1. **Start working on a ticket**:

   ```bash
   ./rig work proj-123
   ```

   This creates:
   - Git worktree at `~/src/your-org/your-repo/proj/proj-123`
   - Markdown note with JIRA details
   - Tmux session with note/code/terminal windows
   - Daily note entry with timestamp

2. **Export your work timeline**:

   ```bash
   ./rig timeline proj-123
   ```

3. **Sync latest information**:
   ```bash
   ./rig sync proj-123
   ```

## Configuration

The CLI uses TOML configuration at `~/.config/rig/config.toml`:

### Single Repository Configuration

```toml
[vault]
path = "~/Documents/Second Brain"
templates_dir = "templates"
areas_dir = "Areas/Work"
daily_dir = "Daily"
default_subdir = "Tickets"    # Default subdir for ticket notes
incident_subdir = "Incidents" # Subdir for incident tickets
hack_subdir = "Hacks"         # Subdir for hack sessions

[repository]
owner = "myorg"
name = "myrepo"
base_path = "~/src"
base_branch = "main"

[history]
database_path = "~/.histdb/zsh-history.db"
ignore_patterns = ["ls", "cd", "pwd", "clear"]

[jira]
enabled = true
mode = "acli"  # or "api" - see Jira Configuration section
cli_command = "acli"

[tmux]
session_prefix = ""

[[tmux.windows]]
name = "note"
command = "nvim {note_path}"

[[tmux.windows]]
name = "code"
command = "nvim"
working_dir = "{worktree_path}"

[[tmux.windows]]
name = "term"
working_dir = "{worktree_path}"
```

### Multi-Repository Configuration

For working with multiple repositories, use the `repositories` table with `ticket_types` to route tickets:

```toml
[vault]
path = "~/Documents/Second Brain"
templates_dir = "templates"
areas_dir = "Areas/Work"
daily_dir = "Daily"
default_subdir = "Tickets"
incident_subdir = "Incidents"
hack_subdir = "Hacks"

# Define multiple repositories
[repositories.main-repo]
owner = "myorg"
name = "main-service"
base_path = "~/src/myorg"
base_branch = "main"

[repositories.infra-repo]
owner = "myorg"
name = "infrastructure"
base_path = "~/src/myorg"
base_branch = "main"

# Map ticket prefixes to repositories
[ticket_types]
proj = "main-repo"
feat = "main-repo"
ops = "infra-repo"
infra = "infra-repo"

[history]
database_path = "~/.histdb/zsh-history.db"
ignore_patterns = ["ls", "cd", "pwd", "clear"]

[jira]
enabled = true
mode = "acli"  # or "api" - see Jira Configuration section
cli_command = "acli"

[tmux]
session_prefix = ""

[[tmux.windows]]
name = "note"
command = "nvim {note_path}"

[[tmux.windows]]
name = "code"
command = "nvim"
working_dir = "{worktree_path}"

[[tmux.windows]]
name = "term"
working_dir = "{worktree_path}"
```

With multi-repo config, `rig work proj-123` routes to `main-repo` while `rig work ops-456` routes to `infra-repo`.

### Jira Configuration

Rig supports two modes for fetching Jira ticket information: direct API access (recommended) and ACLI (legacy).

#### API Mode (Recommended)

Direct API integration provides faster performance and richer metadata without external dependencies.

1. **Create an API token** at https://id.atlassian.com/manage/api-tokens

2. **Set the token as an environment variable** (preferred):
   ```bash
   export JIRA_TOKEN="your-api-token"
   ```

3. **Configure rig**:
   ```toml
   [jira]
   enabled = true
   mode = "api"
   base_url = "https://your-domain.atlassian.net"
   email = "your-email@example.com"
   # token can be set via JIRA_TOKEN env var (preferred) or token field

   [jira.custom_fields]
   story_points = "customfield_10016"
   ```

   To find your custom field IDs, use the Jira REST API or check your Jira admin settings.

#### ACLI Mode (Legacy)

For users who prefer the Atlassian CLI tool:

```toml
[jira]
enabled = true
mode = "acli"
cli_command = "acli"  # path to acli binary if not in PATH
```

Ensure `acli` is installed and configured with your Jira credentials before using this mode.

## Commands Reference

### Core Workflow

#### `rig work <ticket>`

Start complete workflow for a ticket.

**Example:**

```bash
rig work proj-123
```

**What it does:**

- Creates git worktree and branch
- Fetches JIRA ticket details (if configured)
- Creates Markdown note from template
- Updates daily note with timestamp
- Launches tmux session with configured windows

#### `rig hack <name>`

Lightweight workflow for non-ticket work (experiments, spikes, etc.).

**Example:**

```bash
rig hack winter-cleanup
rig hack --notes experiment-auth
```

**Options:**

- `--notes` - Also create an Markdown note for the hack session

**What it does:**

- Creates git worktree at `hack/<name>` with branch `hack/<name>`
- Creates tmux session
- Skips JIRA integration (no ticket needed)
- Optionally creates Markdown note with `--notes` flag

#### `rig list`

Show all worktrees and tmux sessions across configured repositories.

**Options:**

- `--worktrees` - Show only worktrees
- `--sessions` - Show only tmux sessions

#### `rig clean`

Remove old worktrees and associated tmux sessions.

**Options:**

- `--dry-run` - Show what would be removed without removing
- `--force` - Skip confirmation prompts

#### `rig timeline <ticket>`

Generate and export command timeline to Markdown.

**Options:**

- `--since "2025-08-10 09:00"` - Start time filter
- `--until "2025-08-10 18:00"` - End time filter
- `--failed-only` - Show only failed commands
- `--directory /path` - Filter by directory
- `--limit 1000` - Max commands to retrieve
- `--output file.md` - Write to file instead of note
- `--no-update` - Only output to console

**Examples:**

```bash
rig timeline proj-123
rig timeline proj-123 --since "2025-08-10" --failed-only
rig timeline proj-123 --output /tmp/timeline.md
```

### Session Management

#### `rig session list`

List all active tmux sessions.

#### `rig session attach <ticket>`

Attach to existing tmux session for a ticket.

#### `rig session kill <ticket>`

Kill tmux session for a ticket.

### History Analysis

#### `rig history query [pattern]`

Query command history database.

**Options:**

- `--since "2025-08-10"` - Start time filter
- `--until "2025-08-10"` - End time filter
- `--directory /path` - Filter by directory
- `--session name` - Filter by session name
- `--failed-only` - Show only failed commands
- `--limit 50` - Max results to show

**Examples:**

```bash
rig history query "git"
rig history query --since "2025-08-10" --failed-only
rig history query --directory "/Users/me/src/myproject"
```

#### `rig history info`

Show information about the history database.

### Synchronization

#### `rig sync <ticket>`

Update ticket note with fresh JIRA information and daily notes.

**Options:**

- `--jira` - Force refresh of JIRA information
- `--daily` - Update today's daily note only
- `--force` - Force update even if recently modified

#### `rig sync --daily`

Update today's daily note.

### Configuration

#### `rig config --show`

Display current configuration.

#### `rig config --init`

Create default configuration file.

## Prerequisites

### Required Tools

- **Git**: For repository and worktree management
- **Tmux**: For session management
- **Neovim**: For editing (configurable)

### Optional Integrations

- **JIRA CLI**: `acli` or similar for ticket metadata
- **History Database**: zsh-histdb or atuin for command tracking
- **Obsidian**: For note management and templates

### Directory Structure

Your Markdown vault should have this structure (subdirs are configurable):

```
Second Brain/
├── templates/
│   └── Jira.md              # Template for tickets
├── Areas/Work/              # Configurable via areas_dir
│   ├── Tickets/             # Configurable via default_subdir
│   ├── Incidents/           # Configurable via incident_subdir
│   └── Hacks/               # Configurable via hack_subdir
└── Daily/                   # Daily notes (YYYY-MM-DD.md)
```

## Architecture

### Project Structure

```
main/
├── cmd/              # CLI commands (with comprehensive test coverage)
│   ├── clean.go      # Worktree and session cleanup
│   ├── config.go     # Configuration management
│   ├── hack.go       # Lightweight non-ticket workflow
│   ├── history.go    # History database queries
│   ├── work.go       # Ticket workflow start
│   ├── list.go       # List worktrees and sessions
│   ├── root.go       # Root command setup
│   ├── session.go    # Tmux session management
│   ├── sync.go       # Markdown note synchronization
│   ├── timeline.go   # Command history export
│   └── *_test.go     # Unit tests for each command
├── pkg/              # Core packages (with unit tests)
│   ├── config/       # Configuration handling with Viper
│   ├── git/          # Git worktree operations (mock-based testing)
│   ├── history/      # SQLite history queries (zsh-histdb + atuin)
│   ├── jira/         # JIRA integration via CLI (acli)
│   ├── obsidian/     # Markdown note/template management
│   └── tmux/         # Tmux session automation
├── go.mod            # Dependencies
└── main.go           # Entry point
```

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/mattn/go-sqlite3` - SQLite database access
- `gopkg.in/yaml.v3` - YAML parsing

## Development

### Building

```bash
cd main/
go build -o rig
```

### Testing

The project has comprehensive test coverage across all packages.

```bash
cd main/

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -run TestParseTicket ./cmd/...

# Run tests for a specific package
go test ./pkg/git/...
```

#### Test Architecture

- **Table-driven tests**: Most tests use Go's table-driven pattern for comprehensive coverage
- **Mock-based unit tests**: `pkg/git` uses `CommandRunner` interface for mocking git commands
- **Integration tests**: Some tests create real git repos in temp directories (skipped if git unavailable)

#### Test Coverage by Package

- `cmd/` - 9 test files covering all commands except root.go
- `pkg/config/` - Configuration loading and validation
- `pkg/git/` - Mock-based worktree operations testing
- `pkg/history/` - Database schema detection and query building
- `pkg/jira/` - JIRA CLI output parsing
- `pkg/obsidian/` - Note and template management
- `pkg/tmux/` - Session parsing and management

### Adding New Commands

1. Create new command file in `cmd/`
2. Add command to root command in `init()`
3. Implement business logic in appropriate `pkg/` package
4. Add corresponding `_test.go` file with table-driven tests

## Migration from Bash Script

This CLI tool replaces the original `rig.sh` bash script with the following improvements:

### ✅ **Enhanced Features**

- **Better error handling**: Graceful degradation and detailed error messages
- **Rich configuration**: YAML-based with validation and path expansion
- **Advanced querying**: SQLite database integration with filtering
- **Timeline export**: Formatted command history with metadata
- **Extensible architecture**: Easy to add new commands and integrations

### 🔄 **Migration Path**

1. Install and configure the Go CLI
2. Test with existing tickets to ensure compatibility
3. Gradually replace bash script usage
4. Remove bash script once comfortable with CLI

### 🚀 **New Capabilities**

- Command history analysis and export
- Advanced tmux session management
- Configurable window layouts
- Multiple history database support
- Rich timeline formatting

## Contributing

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Submit a pull request

### Code Structure

- Follow Go best practices
- Use the existing package structure
- Add comprehensive error handling
- Include appropriate logging

## License

[Add your license here]

## Support

For issues and questions:

1. Check existing issues in GitHub
2. Create new issue with detailed description
3. Include configuration and error logs

---

**Note**: This tool is designed for Developer workflow automation and integrates with multiple external tools. Ensure all prerequisites are installed and configured for full functionality.
# dummy
