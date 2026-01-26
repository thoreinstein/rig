package cmd

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/tmux"
)

// sessionCmd represents the session command
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage tmux sessions",
	Long: `Manage tmux sessions for workflow tickets.

This command provides subcommands to list, attach, and manage tmux sessions
created by the rig workflow.`,
}

// sessionListCmd lists all tmux sessions
var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tmux sessions",
	Long:  `List all active tmux sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSessionListCommand()
	},
}

// sessionAttachCmd attaches to a tmux session
var sessionAttachCmd = &cobra.Command{
	Use:   "attach <ticket>",
	Short: "Attach to a tmux session for a ticket",
	Long:  `Attach to an existing tmux session for the specified ticket.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSessionAttachCommand(args[0])
	},
}

// sessionKillCmd kills a tmux session
var sessionKillCmd = &cobra.Command{
	Use:   "kill <ticket>",
	Short: "Kill a tmux session for a ticket",
	Long:  `Kill the tmux session associated with the specified ticket.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSessionKillCommand(args[0])
	},
}

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionAttachCmd)
	sessionCmd.AddCommand(sessionKillCmd)
}

func runSessionListCommand() error {
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, nil, verbose)
	sessions, err := sessionManager.ListSessions()
	if err != nil {
		return errors.Wrap(err, "failed to list sessions")
	}

	if len(sessions) == 0 {
		fmt.Println("No tmux sessions found.")
		return nil
	}

	fmt.Println("Active tmux sessions:")
	for i, session := range sessions {
		prefix := "  "
		if i == 0 {
			prefix = "→ "
		}
		fmt.Printf("%s%s\n", prefix, session)
	}

	return nil
}

func runSessionAttachCommand(ticket string) error {
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Parse ticket to handle optional project prefix
	ticketInfo, err := parseTicket(ticket)
	if err != nil {
		return err
	}

	sessionID := ticketInfo.SessionID()

	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, nil, verbose)
	sessionName := sessionManager.GetSessionName(sessionID)

	// Check if session exists
	if !sessionManager.SessionExists(sessionName) {
		return errors.Newf("tmux session '%s' does not exist for ticket '%s'", sessionName, ticket)
	}

	if verbose {
		fmt.Printf("Attaching to session: %s\n", sessionName)
	}

	return sessionManager.AttachToSession(sessionName)
}

func runSessionKillCommand(ticket string) error {
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Parse ticket to handle optional project prefix
	ticketInfo, err := parseTicket(ticket)
	if err != nil {
		return err
	}

	sessionID := ticketInfo.SessionID()

	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, nil, verbose)

	if verbose {
		fmt.Printf("Killing session for ticket: %s\n", ticket)
	}

	err = sessionManager.KillSession(sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			fmt.Printf("Session for ticket '%s' does not exist.\n", ticket)
			return nil
		}
		return errors.Wrap(err, "failed to kill session")
	}

	fmt.Printf("✓ Session for ticket '%s' killed successfully.\n", ticket)
	return nil
}
