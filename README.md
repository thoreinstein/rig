# SRE Workflow CLI

A modern, extensible Go CLI tool for Site Reliability Engineering workflow automation. This tool replaces a complex bash script with a maintainable, feature-rich application that integrates with Git worktrees, Tmux sessions, Obsidian documentation, and command history tracking.

## Features

### 🚀 **Complete Workflow Automation**
- **Ticket-based workflows**: Initialize complete work environments with a single command
- **Git worktree integration**: Automatic isolated workspaces per ticket
- **Tmux session management**: Configurable multi-window terminal sessions
- **Obsidian note creation**: Template-based documentation with JIRA integration
- **Command history tracking**: SQLite-based timeline export and analysis

### 🛠️ **Powerful CLI Interface**
```bash
sre init <ticket>              # Initialize complete workflow
sre session list/attach/kill   # Manage tmux sessions
sre timeline <ticket>          # Export command history timeline
sre history query [pattern]    # Query command database
sre sync <ticket>              # Update notes and JIRA info
sre config --show/--init       # Manage configuration
```

### 🔧 **Integrations**
- **Git**: Automatic worktree and branch creation with base branch detection
- **JIRA**: Ticket metadata fetching via configurable CLI tools (`acli`)
- **Obsidian**: Rich note templates, daily note updates, timeline export
- **Tmux**: Session automation with environment variables and window layouts
- **History Databases**: Support for both zsh-histdb and atuin SQLite schemas

## Quick Start

### Installation

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd sre
   ```

2. **Build the CLI**:
   ```bash
   cd main/
   go build -o sre
   ```

3. **Initialize configuration**:
   ```bash
   ./sre config --init
   ```

4. **Edit configuration** at `~/.config/sre/config.yaml`:
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
   ./sre init fraas-25857
   ```
   This creates:
   - Git worktree at `~/src/your-org/your-repo/fraas/fraas-25857`
   - Obsidian note with JIRA details
   - Tmux session with note/code/terminal windows
   - Daily note entry with timestamp

2. **Export your work timeline**:
   ```bash
   ./sre timeline fraas-25857
   ```

3. **Sync latest information**:
   ```bash
   ./sre sync fraas-25857
   ```

## Configuration

The CLI uses YAML configuration at `~/.config/sre/config.yaml`:

```yaml
vault:
  path: "~/Documents/Second Brain"
  templates_dir: "templates"
  areas_dir: "Areas/Ping Identity"
  daily_dir: "Daily"

repository:
  owner: "myorg"
  name: "myrepo"
  base_path: "~/src"
  base_branch: "main"

history:
  database_path: "~/.histdb/zsh-history.db"
  ignore_patterns: ["ls", "cd", "pwd", "clear"]

jira:
  enabled: true
  cli_command: "acli"

tmux:
  session_prefix: ""
  windows:
    - name: "note"
      command: "nvim {note_path}"
    - name: "code" 
      command: "nvim"
      working_dir: "{worktree_path}"
    - name: "term"
      working_dir: "{worktree_path}"
```

## Commands Reference

### Core Workflow

#### `sre init <ticket>`
Initialize complete workflow for a ticket.

**Example:**
```bash
sre init fraas-25857
```

**What it does:**
- Creates git worktree and branch
- Fetches JIRA ticket details (if configured)
- Creates Obsidian note from template
- Updates daily note with timestamp
- Launches tmux session with configured windows

#### `sre timeline <ticket>`
Generate and export command timeline to Obsidian.

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
sre timeline fraas-25857
sre timeline fraas-25857 --since "2025-08-10" --failed-only
sre timeline fraas-25857 --output /tmp/timeline.md
```

### Session Management

#### `sre session list`
List all active tmux sessions.

#### `sre session attach <ticket>`
Attach to existing tmux session for a ticket.

#### `sre session kill <ticket>`
Kill tmux session for a ticket.

### History Analysis

#### `sre history query [pattern]`
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
sre history query "git"
sre history query --since "2025-08-10" --failed-only
sre history query --directory "/Users/me/src/myproject"
```

#### `sre history info`
Show information about the history database.

### Synchronization

#### `sre sync <ticket>`
Update ticket note with fresh JIRA information and daily notes.

**Options:**
- `--jira` - Force refresh of JIRA information
- `--daily` - Update today's daily note only
- `--force` - Force update even if recently modified

#### `sre sync --daily`
Update today's daily note.

### Configuration

#### `sre config --show`
Display current configuration.

#### `sre config --init`
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
Your Obsidian vault should have this structure:
```
Second Brain/
├── templates/
│   └── Jira.md              # Template for JIRA tickets
├── Areas/Ping Identity/
│   ├── Jira/               # Generated ticket notes
│   └── Incidents/          # Incident notes
└── Daily/                  # Daily notes (YYYY-MM-DD.md)
```

## Architecture

### Project Structure
```
main/
├── cmd/              # CLI commands
│   ├── config.go     # Configuration management
│   ├── init.go       # Ticket workflow initialization  
│   ├── session.go    # Tmux session management
│   ├── timeline.go   # Command history export
│   ├── history.go    # History database queries
│   ├── sync.go       # Obsidian note synchronization
│   └── root.go       # Root command setup
├── pkg/              # Core packages
│   ├── config/       # Configuration handling with Viper
│   ├── git/          # Git worktree operations
│   ├── obsidian/     # Obsidian note management
│   ├── tmux/         # Tmux session automation
│   ├── history/      # SQLite history database integration
│   └── jira/         # JIRA API integration via acli
├── go.mod           # Dependencies
└── main.go          # Entry point
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
go build -o sre
```

### Testing
```bash
cd main/
go test ./...
```

### Adding New Commands
1. Create new command file in `cmd/`
2. Add command to root command in `init()`
3. Implement business logic in appropriate `pkg/` package

## Migration from Bash Script

This CLI tool replaces the original `sre.sh` bash script with the following improvements:

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

**Note**: This tool is designed for SRE workflow automation and integrates with multiple external tools. Ensure all prerequisites are installed and configured for full functionality.