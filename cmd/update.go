package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

const (
	repoOwner = "thoreinstein"
	repoName  = "sre"
)

var (
	updateCheck bool
	updateForce bool
	updatePre   bool
	updateYes   bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update sre to the latest version",
	Long: `Check for and install the latest version of sre from GitHub releases.

This command connects to GitHub to check for newer releases. If a newer
version is found, it downloads the appropriate binary for your platform
and replaces the current executable.

The update process validates the download using checksums to ensure
integrity before replacing the binary.

Examples:
  sre update           # Check and update (with confirmation)
  sre update --check   # Only check for updates, don't install
  sre update --yes     # Update without confirmation prompt
  sre update --force   # Force reinstall even if on latest version
  sre update --pre     # Include pre-release versions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdateCommand(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().BoolVarP(&updateCheck, "check", "c", false,
		"Check for updates without installing")
	updateCmd.Flags().BoolVarP(&updateForce, "force", "f", false,
		"Force update even if already on latest version")
	updateCmd.Flags().BoolVarP(&updatePre, "pre", "p", false,
		"Include pre-release versions")
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false,
		"Skip confirmation prompt")
}

func runUpdateCommand(ctx context.Context) error {
	if verbose {
		fmt.Printf("Current version: %s\n", Version)
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	}

	// Create GitHub source
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return fmt.Errorf("failed to create GitHub source: %w", err)
	}

	// Create updater with checksum validation
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	// Detect latest release
	if verbose {
		fmt.Println("Checking for updates...")
	}

	repo := selfupdate.NewRepositorySlug(repoOwner, repoName)

	var latest *selfupdate.Release
	var found bool

	if updatePre {
		// Include pre-releases by getting all releases and finding the latest
		latest, found, err = updater.DetectLatest(ctx, repo)
	} else {
		latest, found, err = updater.DetectLatest(ctx, repo)
	}

	if err != nil {
		return fmt.Errorf("failed to detect latest version: %w", err)
	}

	if !found {
		return fmt.Errorf("no release found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if verbose {
		fmt.Printf("Latest version: %s\n", latest.Version())
		fmt.Printf("Release URL: %s\n", latest.URL)
		fmt.Printf("Asset: %s\n", latest.AssetName)
	}

	// Compare versions
	currentVersion := Version

	// Handle "dev" version - always consider it older than any release
	isDevVersion := currentVersion == "dev"

	if !isDevVersion && latest.LessOrEqual(currentVersion) && !updateForce {
		fmt.Printf("Current version (%s) is up to date\n", currentVersion)
		return nil
	}

	// Report available update
	if isDevVersion {
		fmt.Printf("Development version detected, latest release is %s\n", latest.Version())
	} else {
		fmt.Printf("Update available: %s -> %s\n", currentVersion, latest.Version())
	}

	// If --check, just report and exit
	if updateCheck {
		return nil
	}

	// Prompt for confirmation unless --yes
	if !updateYes {
		if !confirmUpdate(currentVersion, latest.Version()) {
			fmt.Println("Update cancelled")
			return nil
		}
	}

	// Get executable path
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	if verbose {
		fmt.Printf("Executable path: %s\n", exe)
		fmt.Println("Downloading update...")
	}

	// Perform update
	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
		return fmt.Errorf("failed to update binary: %w", err)
	}

	fmt.Printf("Successfully updated to version %s\n", latest.Version())
	return nil
}

// confirmUpdate prompts the user for confirmation before updating.
func confirmUpdate(currentVersion, newVersion string) bool {
	var prompt string
	if currentVersion == "dev" {
		prompt = fmt.Sprintf("Update sre from dev to %s? [y/N]: ", newVersion)
	} else {
		prompt = fmt.Sprintf("Update sre from %s to %s? [y/N]: ", currentVersion, newVersion)
	}

	fmt.Print(prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
