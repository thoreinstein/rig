package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"thoreinstein.com/sre/pkg/config"
	"thoreinstein.com/sre/pkg/git"
	"thoreinstein.com/sre/pkg/jira"
	"thoreinstein.com/sre/pkg/obsidian"
	"thoreinstein.com/sre/pkg/tmux"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init <ticket>",
	Short: "Initialize SRE workflow for a ticket",
	Long: `Initialize the complete SRE workflow for a given ticket.

This command performs the following actions:
- Parses ticket type and number
- Creates git worktree and branch
- Creates/updates Obsidian note with JIRA integration
- Updates daily note with log entry  
- Creates tmux session with configured windows

Examples:
  sre init fraas-25857
  sre init cre-123
  sre init incident-456`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInitCommand(args[0])
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// TicketInfo holds parsed ticket information
type TicketInfo struct {
	Full   string
	Type   string
	Number string
}

// parseTicket parses a ticket string into type and number components
func parseTicket(ticket string) (*TicketInfo, error) {
	// Match pattern: TYPE-NUMBER (e.g., fraas-25857, cre-123)
	re := regexp.MustCompile(`^([a-zA-Z]+)-([0-9]+)$`)
	matches := re.FindStringSubmatch(ticket)
	
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid ticket format. Expected format: TYPE-NUMBER (e.g., fraas-25857)")
	}
	
	return &TicketInfo{
		Full:   ticket,
		Type:   strings.ToLower(matches[1]),
		Number: matches[2],
	}, nil
}

func runInitCommand(ticket string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Parse ticket
	ticketInfo, err := parseTicket(ticket)
	if err != nil {
		return err
	}

	// Get repository config for this ticket type
	repoConfig := cfg.GetRepoForTicketType(ticketInfo.Type)
	repoPath := cfg.GetRepositoryPathForRepo(repoConfig)

	if verbose {
		fmt.Printf("Starting workflow for ticket: %s\n", ticketInfo.Full)
		fmt.Printf("  Type: %s\n", ticketInfo.Type)
		fmt.Printf("  Number: %s\n", ticketInfo.Number)
		fmt.Printf("  Repository: %s/%s\n", repoConfig.Owner, repoConfig.Name)
	}

	// Step 1: Create git worktree
	if verbose {
		fmt.Println("Creating git worktree...")
	}
	gitManager := git.NewWorktreeManager(repoPath, repoConfig.BaseBranch, verbose)
	worktreePath, err := gitManager.CreateWorktree(ticketInfo.Type, ticketInfo.Full)
	if err != nil {
		return fmt.Errorf("failed to create git worktree: %w", err)
	}
	fmt.Printf("✓ Git worktree created at: %s\n", worktreePath)
	
	// Step 2: Fetch JIRA details (if enabled)
	var jiraInfo *obsidian.JiraInfo
	if cfg.Jira.Enabled {
		if verbose {
			fmt.Println("Fetching JIRA details...")
		}
		jiraClient := jira.NewClient(cfg.Jira.CliCommand, verbose)
		jiraInfo, err = jiraClient.FetchTicketDetails(ticketInfo.Full)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Could not fetch JIRA details: %v\n", err)
			}
			// Don't fail the entire process if JIRA fetch fails
			jiraInfo = nil
		} else {
			fmt.Println("✓ JIRA details fetched successfully")
		}
	}
	
	// Step 3: Create/update Obsidian note
	if verbose {
		fmt.Println("Creating Obsidian note...")
	}
	vaultSubdir := cfg.GetVaultSubdir(ticketInfo.Type)
	noteManager := obsidian.NewNoteManager(
		cfg.Vault.Path,
		cfg.Vault.TemplatesDir,
		cfg.Vault.AreasDir,
		cfg.Vault.DailyDir,
		verbose,
	)
	noteManager.SetVaultSubdir(vaultSubdir)
	notePath, err := noteManager.CreateTicketNote(ticketInfo.Type, ticketInfo.Full, jiraInfo)
	if err != nil {
		return fmt.Errorf("failed to create Obsidian note: %w", err)
	}
	fmt.Printf("✓ Obsidian note created at: %s\n", notePath)
	
	// Step 4: Update daily note
	if verbose {
		fmt.Println("Updating daily note...")
	}
	err = noteManager.UpdateDailyNote(ticketInfo.Full)
	if err != nil {
		// Don't fail if daily note update fails
		if verbose {
			fmt.Printf("Warning: Could not update daily note: %v\n", err)
		}
	} else {
		fmt.Println("✓ Daily note updated")
	}
	
	// Step 5: Create tmux session
	if verbose {
		fmt.Println("Creating tmux session...")
	}
	
	// Convert config windows to tmux windows
	var tmuxWindows []tmux.WindowConfig
	for _, window := range cfg.Tmux.Windows {
		tmuxWindows = append(tmuxWindows, tmux.WindowConfig{
			Name:       window.Name,
			Command:    window.Command,
			WorkingDir: window.WorkingDir,
		})
	}
	
	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, tmuxWindows, verbose)
	err = sessionManager.CreateSession(ticketInfo.Full, worktreePath, notePath)
	if err != nil {
		// Don't fail the entire process if tmux session creation fails
		if verbose {
			fmt.Printf("Warning: Could not create tmux session: %v\n", err)
		}
		fmt.Println("Warning: Tmux session creation failed, but other steps completed successfully")
	} else {
		fmt.Println("✓ Tmux session created successfully")
	}
	
	fmt.Printf("\n🎉 Workflow initialization for %s completed successfully!\n", ticketInfo.Full)
	fmt.Printf("Worktree: %s\n", worktreePath)
	fmt.Printf("Note: %s\n", notePath)
	
	return nil
}