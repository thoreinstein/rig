# Tech Stack: Rig

## Core Technologies
- **Programming Language:** [Go](https://go.dev/) (v1.25.5) - Chosen for its strong performance, excellent concurrency support, and ease of creating single-binary CLI tools.
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra) - The industry standard for building powerful and modern Go CLI applications.
- **Configuration Management:** [Viper](https://github.com/spf13/viper) - A complete configuration solution for Go applications, supporting multiple formats and environment variables.
- **Database:** [modernc.org/sqlite](https://modernc.org/sqlite) - A CGO-free SQLite implementation for Go, ensuring easy cross-compilation and zero external dependencies for data persistence.

## External Integrations
- **Version Control:** [Git](https://git-scm.com/) - Specifically utilizing git-worktree for managing isolated development environments.
- **Terminal Multiplexer:** [Tmux](https://github.com/tmux/tmux) - Used for session management and terminal workspace automation.
- **Project Management:** [JIRA](https://www.atlassian.com/software/jira) - Integration via both direct REST API and the Atlassian CLI (ACLI).
- **Knowledge Management:** [Obsidian](https://obsidian.md/) - Focus on Markdown-based note management and template-driven documentation.
- **Platform CLI:** [GitHub CLI (gh)](https://cli.github.com/) - Integrated for repository management, pull requests, and GitHub-specific workflows.
- **Platform API:** [GitHub API](https://docs.github.com/en/rest) - For deep integration with GitHub services.

## Key Libraries
- `github.com/cockroachdb/errors`: For rich, structured error handling.
- `github.com/creativeprojects/go-selfupdate`: To enable seamless application self-updates.
- `github.com/zalando/go-keyring`: For secure storage of credentials in the system's native keyring.
