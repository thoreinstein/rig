package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SessionManager handles Tmux session operations
type SessionManager struct {
	SessionPrefix string
	Windows       []WindowConfig
	Verbose       bool
}

// WindowConfig represents a tmux window configuration
type WindowConfig struct {
	Name       string
	Command    string
	WorkingDir string
}

// NewSessionManager creates a new SessionManager
func NewSessionManager(sessionPrefix string, windows []WindowConfig, verbose bool) *SessionManager {
	return &SessionManager{
		SessionPrefix: sessionPrefix,
		Windows:       windows,
		Verbose:       verbose,
	}
}

// CreateSession creates a new tmux session for a ticket
func (sm *SessionManager) CreateSession(ticket, worktreePath, notePath string) error {
	sessionName := sm.getSessionName(ticket)

	// Check if session already exists
	if sm.sessionExists(sessionName) {
		if sm.Verbose {
			fmt.Printf("Tmux session '%s' already exists. Attaching...\n", sessionName)
		}
		return sm.attachToSession(sessionName)
	}

	if sm.Verbose {
		fmt.Printf("Creating tmux session '%s'...\n", sessionName)
	}

	// Verify worktree directory exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return fmt.Errorf("worktree path does not exist: %s", worktreePath)
	}

	// Create session with first window
	err := sm.createInitialSession(sessionName, worktreePath)
	if err != nil {
		return fmt.Errorf("failed to create initial session: %w", err)
	}

	// Create additional windows
	err = sm.createWindows(sessionName, worktreePath, notePath)
	if err != nil {
		return fmt.Errorf("failed to create windows: %w", err)
	}

	// Set environment variables for the session
	err = sm.setEnvironmentVars(sessionName, ticket, worktreePath)
	if err != nil {
		return fmt.Errorf("failed to set environment variables: %w", err)
	}

	// Start on the first window (note)
	err = sm.selectWindow(sessionName, 1)
	if err != nil {
		return fmt.Errorf("failed to select first window: %w", err)
	}

	// Attach to the session if we're in a tmux session, otherwise switch
	return sm.attachToSession(sessionName)
}

// GetSessionName returns the full session name with prefix
func (sm *SessionManager) GetSessionName(ticket string) string {
	if sm.SessionPrefix != "" {
		return sm.SessionPrefix + ticket
	}
	return ticket
}

// getSessionName is a private helper method
func (sm *SessionManager) getSessionName(ticket string) string {
	return sm.GetSessionName(ticket)
}

// SessionExists checks if a tmux session exists
func (sm *SessionManager) SessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

// sessionExists is a private helper method
func (sm *SessionManager) sessionExists(sessionName string) bool {
	return sm.SessionExists(sessionName)
}

// createInitialSession creates the initial tmux session
func (sm *SessionManager) createInitialSession(sessionName, worktreePath string) error {
	// Determine the initial working directory (use vault path if available from first window)
	var initialDir string
	if len(sm.Windows) > 0 && sm.Windows[0].WorkingDir != "" {
		initialDir = sm.expandPath(sm.Windows[0].WorkingDir, worktreePath, "")
	} else {
		initialDir = worktreePath
	}

	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", initialDir)

	if sm.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// createWindows creates additional windows based on configuration
func (sm *SessionManager) createWindows(sessionName, worktreePath, notePath string) error {
	for i, window := range sm.Windows {
		windowTarget := fmt.Sprintf("%s:%d", sessionName, i+1)

		if i == 0 {
			// Rename the first window that was created with the session
			err := sm.renameWindow(windowTarget, window.Name)
			if err != nil {
				return fmt.Errorf("failed to rename first window: %w", err)
			}
		} else {
			// Create new window
			workingDir := sm.expandPath(window.WorkingDir, worktreePath, notePath)
			if workingDir == "" {
				workingDir = worktreePath
			}

			err := sm.createWindow(sessionName, i+1, window.Name, workingDir)
			if err != nil {
				return fmt.Errorf("failed to create window %d: %w", i+1, err)
			}
		}

		// Send command to window if specified
		if window.Command != "" {
			command := sm.expandPath(window.Command, worktreePath, notePath)
			err := sm.sendCommand(windowTarget, command)
			if err != nil {
				return fmt.Errorf("failed to send command to window %s: %w", window.Name, err)
			}
		}
	}

	return nil
}

// expandPath expands template variables in paths and commands
func (sm *SessionManager) expandPath(template, worktreePath, notePath string) string {
	result := template
	result = strings.ReplaceAll(result, "{worktree_path}", worktreePath)
	result = strings.ReplaceAll(result, "{note_path}", notePath)

	// Expand ~ to home directory
	if strings.HasPrefix(result, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			result = filepath.Join(homeDir, result[2:])
		}
	}

	return result
}

// renameWindow renames a tmux window
func (sm *SessionManager) renameWindow(windowTarget, name string) error {
	cmd := exec.Command("tmux", "rename-window", "-t", windowTarget, name)

	if sm.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// createWindow creates a new tmux window
func (sm *SessionManager) createWindow(sessionName string, windowNum int, name, workingDir string) error {
	windowTarget := fmt.Sprintf("%s:%d", sessionName, windowNum)

	cmd := exec.Command("tmux", "new-window", "-t", windowTarget, "-n", name, "-c", workingDir)

	if sm.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// sendCommand sends a command to a tmux window
func (sm *SessionManager) sendCommand(windowTarget, command string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", windowTarget, command, "Enter")

	if sm.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// setEnvironmentVars sets environment variables for the tmux session
func (sm *SessionManager) setEnvironmentVars(sessionName, ticket, worktreePath string) error {
	vars := map[string]string{
		"SRE_TICKET":   ticket,
		"SRE_WORKTREE": worktreePath,
	}

	for key, value := range vars {
		cmd := exec.Command("tmux", "set-environment", "-t", sessionName, key, value)

		if sm.Verbose {
			fmt.Printf("Setting %s=%s\n", key, value)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set %s: %w", key, err)
		}
	}

	return nil
}

// selectWindow selects a specific window in the session
func (sm *SessionManager) selectWindow(sessionName string, windowNum int) error {
	windowTarget := fmt.Sprintf("%s:%d", sessionName, windowNum)

	cmd := exec.Command("tmux", "select-window", "-t", windowTarget)

	if sm.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// AttachToSession attaches to or switches to a tmux session
func (sm *SessionManager) AttachToSession(sessionName string) error {
	// Check if we're already in a tmux session
	if os.Getenv("TMUX") != "" {
		// We're in tmux, switch to the session
		cmd := exec.Command("tmux", "switch-client", "-t", sessionName)

		if sm.Verbose {
			fmt.Printf("Switching to session: %s\n", sessionName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		return cmd.Run()
	}

	// We're not in tmux, attach to the session
	cmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if sm.Verbose {
		fmt.Printf("Attaching to session: %s\n", sessionName)
	}

	return cmd.Run()
}

// attachToSession is a private helper method
func (sm *SessionManager) attachToSession(sessionName string) error {
	return sm.AttachToSession(sessionName)
}

// ListSessions returns a list of all tmux sessions
func (sm *SessionManager) ListSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Filter out empty strings
	var result []string
	for _, session := range sessions {
		if strings.TrimSpace(session) != "" {
			result = append(result, session)
		}
	}

	return result, nil
}

// KillSession kills a tmux session
func (sm *SessionManager) KillSession(ticket string) error {
	sessionName := sm.getSessionName(ticket)

	if !sm.sessionExists(sessionName) {
		return fmt.Errorf("session does not exist: %s", sessionName)
	}

	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)

	if sm.Verbose {
		fmt.Printf("Killing session: %s\n", sessionName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}
