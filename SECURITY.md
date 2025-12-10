# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it by opening a GitHub issue or contacting the maintainers directly.

For sensitive vulnerabilities, please do not open a public issue. Instead, contact the maintainers privately.

## Security Model

### Configuration File Trust

The SRE CLI configuration file (`~/.config/sre/config.toml`) is a **trusted file**. It has the ability to:

1. **Execute arbitrary commands** via tmux window configurations (`tmux.windows[].command`)
2. **Specify external binaries** to run (e.g., `jira.cli_command`)
3. **Access file paths** throughout your system

**Important**: Only use configuration files from trusted sources. Never copy configuration from untrusted locations or allow untrusted users to modify your config file.

### Recommended File Permissions

The CLI creates configuration files with `0600` permissions (owner read/write only). If you create or modify config files manually, ensure they have appropriate permissions:

```bash
chmod 600 ~/.config/sre/config.toml
```

### External Tool Dependencies

This tool invokes external binaries:

- `git` - Git operations
- `tmux` - Terminal multiplexer session management
- JIRA CLI (configurable, default: `acli`) - JIRA ticket fetching

Ensure these tools are installed from trusted sources and your `PATH` is configured securely.

### Input Validation

The CLI validates user input:

- **Ticket IDs**: Must match pattern `^[a-zA-Z]+-[0-9]+$` (e.g., `PROJ-123`)
- **Hack names**: Must match pattern `^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`
- **Output paths**: Validated against path traversal and restricted to safe locations

### Database Access

The CLI reads shell history databases (zsh-histdb or atuin) in read-only mode. It uses parameterized SQL queries to prevent injection attacks.

### Notes and Files

Notes and timeline exports may contain:

- Command history (potentially including arguments with sensitive data)
- File paths from your system
- JIRA ticket details

Be mindful when sharing these files.
