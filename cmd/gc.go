package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/events"
	"thoreinstein.com/rig/pkg/orchestration"
)

var (
	gcDryRun  bool
	gcForce   bool
	gcTarget  string
	gcAge     string
	gcArchive bool
)

// gcCmd represents the gc command
var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect old Dolt history",
	Long: `Prune old workflow events and executions from the Dolt databases.

This command identifies data older than the specified age and removes it.
By default, it targets both events and orchestration databases.

Examples:
  rig gc --age 30d       # Prune data older than 30 days
  rig gc --target events # Only prune events
  rig gc --dry-run       # Show what would be pruned without deleting
  rig gc --archive       # Export data to JSON before pruning`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGCCommand(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(gcCmd)

	gcCmd.Flags().BoolVar(&gcDryRun, "dry-run", false, "Show what would be removed without removing")
	gcCmd.Flags().BoolVar(&gcForce, "force", false, "Remove without confirmation prompts")
	gcCmd.Flags().StringVar(&gcTarget, "target", "all", "Target database: events, orchestration, or all")
	gcCmd.Flags().StringVar(&gcAge, "age", "", "Age threshold for pruning (e.g., 30d, 90d). Defaults to config.")
	gcCmd.Flags().BoolVar(&gcArchive, "archive", false, "Export data to JSON before pruning")
}

func runGCCommand(cmdCtx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	targetEvents := gcTarget == "all" || gcTarget == "events"
	targetOrch := gcTarget == "all" || gcTarget == "orchestration"

	if !targetEvents && !targetOrch {
		return errors.Errorf("invalid target: %s. Must be events, orchestration, or all", gcTarget)
	}

	// Respect store enablement configuration to avoid side effects when disabled.
	if targetEvents && !cfg.Events.Enabled {
		return errors.New("events store is disabled in configuration; enable it or choose a different --target")
	}
	if targetOrch && !cfg.Orchestration.Enabled {
		return errors.New("orchestration store is disabled in configuration; enable it or choose a different --target")
	}

	// 1. Determine Cutoffs
	var eventsCutoff, orchCutoff time.Time
	var eventsAge, orchAge string

	if targetEvents {
		eventsCutoff, eventsAge, err = determineEventsCutoff(cfg, gcAge)
		if err != nil {
			return errors.Wrap(err, "failed to determine events cutoff")
		}
	}

	if targetOrch {
		orchCutoff, orchAge, err = determineOrchCutoff(cfg, gcAge)
		if err != nil {
			return errors.Wrap(err, "failed to determine orchestration cutoff")
		}
	}

	// Summarize intent
	if targetEvents && targetOrch {
		if eventsAge == orchAge {
			fmt.Printf("Pruning all data older than %s (%s)\n", eventsAge, eventsCutoff.Format(time.RFC3339))
		} else {
			fmt.Printf("Pruning events older than %s and orchestration older than %s\n", eventsAge, orchAge)
		}
	} else if targetEvents {
		fmt.Printf("Pruning events older than %s (%s)\n", eventsAge, eventsCutoff.Format(time.RFC3339))
	} else {
		fmt.Printf("Pruning orchestration older than %s (%s)\n", orchAge, orchCutoff.Format(time.RFC3339))
	}

	if gcDryRun {
		fmt.Println("Running in DRY-RUN mode. No data will be deleted.")
	}

	// 2. Confirmation
	if !gcForce && !gcDryRun {
		fmt.Printf("Proceed with pruning? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrap(err, "failed to read input")
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	archiveDir := ""
	if gcArchive && !gcDryRun {
		home, err := config.UserHomeDir()
		if err != nil {
			return errors.Wrap(err, "failed to determine home directory for archive")
		}
		archiveDir = filepath.Join(home, ".local", "share", "rig", "archives")
	}

	ctx, cancel := contextWithTimeout(cmdCtx, 30*time.Minute) // GC can be slow
	defer cancel()

	var errs []error

	// 3. Process Events
	if targetEvents {
		if err := processEventsGC(ctx, cfg, eventsCutoff, archiveDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error during events GC: %v\n", err)
			errs = append(errs, errors.Wrap(err, "events GC failed"))
		}
	}

	// 4. Process Orchestration
	if targetOrch {
		if err := processOrchGC(ctx, cfg, orchCutoff, archiveDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error during orchestration GC: %v\n", err)
			errs = append(errs, errors.Wrap(err, "orchestration GC failed"))
		}
	}

	if len(errs) > 0 {
		var combined error
		for _, e := range errs {
			combined = errors.CombineErrors(combined, e)
		}
		return combined
	}

	return nil
}

func determineEventsCutoff(cfg *config.Config, ageStr string) (time.Time, string, error) {
	if ageStr == "" && cfg.Events.RetentionDays > 0 {
		ageStr = fmt.Sprintf("%dd", cfg.Events.RetentionDays)
	}
	if ageStr == "" {
		return time.Time{}, "", errors.New("events age must be specified via --age flag or events.retention_days config")
	}
	days, err := parseAge(ageStr)
	if err != nil {
		return time.Time{}, "", err
	}
	return time.Now().AddDate(0, 0, -days), ageStr, nil
}

func determineOrchCutoff(cfg *config.Config, ageStr string) (time.Time, string, error) {
	if ageStr == "" && cfg.Orchestration.RetentionDays > 0 {
		ageStr = fmt.Sprintf("%dd", cfg.Orchestration.RetentionDays)
	}
	if ageStr == "" {
		return time.Time{}, "", errors.New("orchestration age must be specified via --age flag or orchestration.retention_days config")
	}
	days, err := parseAge(ageStr)
	if err != nil {
		return time.Time{}, "", err
	}
	return time.Now().AddDate(0, 0, -days), ageStr, nil
}

func parseAge(age string) (int, error) {
	age = strings.ToLower(strings.TrimSpace(age))
	if !strings.HasSuffix(age, "d") {
		return 0, errors.Errorf("invalid age format: %q. Expected format like '30d'", age)
	}

	numStr := strings.TrimSuffix(age, "d")
	days, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, errors.Errorf("invalid age number: %q", numStr)
	}

	if days <= 0 {
		return 0, errors.Errorf("age must be a positive number of days")
	}

	return days, nil
}

func processEventsGC(ctx context.Context, cfg *config.Config, cutoff time.Time, archiveDir string) error {
	dm, err := events.NewDatabaseManager(cfg.Events.DataPath, cfg.Events.CommitName, cfg.Events.CommitEmail, verbose)
	if err != nil {
		return err
	}
	defer dm.Close()

	if err := dm.InitDatabase(); err != nil {
		return errors.Wrap(err, "failed to initialize events database")
	}

	if gcArchive && !gcDryRun {
		count, path, err := dm.ExportEventsBeforeCutoff(ctx, cutoff, archiveDir)
		if err != nil {
			return errors.Wrap(err, "archival failed")
		}
		if count > 0 {
			fmt.Printf("[events] Archived %d events to %s\n", count, path)
		}
	}

	res, err := dm.PruneEvents(ctx, cutoff, gcDryRun)
	if err != nil {
		return err
	}

	if gcDryRun {
		fmt.Printf("[events] Would prune %d events\n", res.EventsDeleted)
	} else {
		fmt.Printf("[events] Pruned %d events\n", res.EventsDeleted)
		if res.EventsDeleted > 0 {
			if err := dm.DoltGC(ctx); err != nil {
				return errors.Wrap(err, "Dolt GC failed")
			}
		}
	}

	return nil
}

func processOrchGC(ctx context.Context, cfg *config.Config, cutoff time.Time, archiveDir string) error {
	dm, err := orchestration.NewDatabaseManager(cfg.Orchestration.DataPath, cfg.Orchestration.CommitName, cfg.Orchestration.CommitEmail, verbose)
	if err != nil {
		return err
	}
	defer dm.Close()

	if err := dm.InitDatabase(); err != nil {
		return errors.Wrap(err, "failed to initialize orchestration database")
	}

	if gcArchive && !gcDryRun {
		count, path, err := dm.ExportExecutionsBeforeCutoff(ctx, cutoff, archiveDir)
		if err != nil {
			return errors.Wrap(err, "archival failed")
		}
		if count > 0 {
			fmt.Printf("[orchestration] Archived %d executions to %s\n", count, path)
		}
	}

	res, err := dm.PruneExecutions(ctx, cutoff, gcDryRun)
	if err != nil {
		return err
	}

	if gcDryRun {
		fmt.Printf("[orchestration] Would prune %d executions and %d node states\n", res.ExecutionsPruned, res.NodeStatesPruned)
	} else {
		fmt.Printf("[orchestration] Pruned %d executions and %d node states\n", res.ExecutionsPruned, res.NodeStatesPruned)
		if res.ExecutionsPruned > 0 {
			if err := dm.DoltGC(ctx); err != nil {
				return errors.Wrap(err, "Dolt GC failed")
			}
		}
	}

	return nil
}
