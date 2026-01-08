# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it by emailing the maintainer directly.
Do **not** open a public GitHub issue for security vulnerabilities.

## Security Model

### Trust Boundaries

This CLI tool operates with the following trust assumptions:

1. **Config files are user-controlled and trusted**
   - `~/.config/rig/config.yaml` is created and maintained by the user
   - Commands specified in config (e.g., tmux window commands) are executed as the user
   - Defense-in-depth: Command allowlists limit blast radius if config is compromised

2. **Git repositories are trusted**
   - The tool operates on repositories the user has cloned
   - Worktree paths are validated to prevent path traversal attacks

3. **Shell history databases are trusted**
   - Reads from user's zsh history or histdb
   - Uses parameterized queries where applicable
   - Table names use explicit switch statements (not user input)

4. **Binary updates use checksum validation**
   - Downloaded binaries are validated against `checksums.txt`
   - Checksums are SHA256 hashes published with each GitHub release

### Defense-in-Depth Measures

- **Path traversal protection**: Worktree paths are validated to stay within repository root
- **Restrictive permissions**: Notes directories use 0700 (user-only)
- **Command allowlists**: Tmux commands are validated against regex patterns
- **Checksum validation**: Binary updates are verified against published checksums

## Security Hardening Checklist

When deploying this tool:

- [ ] Review `~/.config/rig/config.yaml` permissions (should be 0600)
- [ ] Verify binary checksum before first run
- [ ] Keep the tool updated (`rig update`)
