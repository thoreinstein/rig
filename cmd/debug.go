package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
)

var debugJSON bool

// debugCmd represents the debug command group
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Diagnostic tools for Rig",
	Long:  `Subcommands for debugging Rig's configuration, plugins, and internal state.`,
}

// debugConfigCmd represents the rig debug config subcommand
var debugConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the final resolved configuration with source attribution and discovery trace",
	RunE:  runDebugConfig,
}

func init() {
	rootCmd.AddCommand(debugCmd)
	debugCmd.AddCommand(debugConfigCmd)

	debugConfigCmd.Flags().BoolVar(&debugJSON, "json", false, "Output in JSON format")
}

func runDebugConfig(cmd *cobra.Command, args []string) error {
	if appLoader == nil {
		return errors.New("configuration loader not initialized")
	}

	// Reload to ensure we have the latest and discovery log is populated
	_, err := appLoader.Load()
	if err != nil {
		return errors.Wrap(err, "failed to reload configuration")
	}

	sources := appLoader.Sources()
	discovery := appLoader.DiscoveryLog()
	violations := appLoader.Violations()

	if debugJSON {
		return outputConfigJSON(sources, discovery, violations)
	}

	return outputConfigHuman(sources, discovery, violations)
}

func outputConfigHuman(sources config.SourceMap, discovery []config.DiscoveryEvent, violations []config.TrustViolation) error {
	// 1. Context
	fmt.Println("DIAGNOSTIC CONTEXT")
	fmt.Println("------------------")
	if appLoader.UserFile() != "" {
		fmt.Printf("User Config:  %s\n", appLoader.UserFile())
	}
	fmt.Println()

	// 2. Discovery Log
	fmt.Println("DISCOVERY LOG")
	fmt.Println("-------------")
	for _, event := range discovery {
		fmt.Printf("[%s] %s\n", event.Tier, event.Message)
	}
	fmt.Println()

	// 3. Effective Config Table
	fmt.Println("EFFECTIVE CONFIGURATION")
	fmt.Println("-----------------------")
	keys := make([]string, 0, len(sources))
	for k := range sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	violationsByKey := make(map[string]config.ViolationReason, len(violations))
	for _, v := range violations {
		violationsByKey[v.Key] = v.Reason
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE\tSOURCE\tPROTECTION")
	fmt.Fprintln(w, "---\t-----\t------\t----------")

	for _, k := range keys {
		entry := sources[k]
		sourceStr := sources.Get(k)

		val := fmt.Sprintf("%v", entry.Value)
		if config.IsSensitiveKey(k) {
			val = "********"
		}

		protection := ""
		if config.IsImmutable(k) {
			protection = "immutable"
		} else if reason, ok := violationsByKey[k]; ok && reason == config.ViolationUntrustedProject {
			protection = "untrusted"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", k, val, sourceStr, protection)
	}
	w.Flush()
	fmt.Println()

	// 4. Violations Summary
	if len(violations) > 0 {
		fmt.Println("TRUST VIOLATIONS")
		fmt.Println("----------------")
		for _, v := range violations {
			fmt.Printf("! %s: %s (File: %s)\n", v.Key, v.Reason, v.File)
		}
	}

	return nil
}

type debugConfigOutput struct {
	Context    map[string]string            `json:"context"`
	Discovery  []config.DiscoveryEvent      `json:"discovery"`
	Config     map[string]configValueOutput `json:"config"`
	Violations []config.TrustViolation      `json:"violations"`
}

type configValueOutput struct {
	Value      interface{} `json:"value"`
	Source     string      `json:"source"`
	File       string      `json:"file,omitempty"`
	Protection string      `json:"protection,omitempty"`
}

func outputConfigJSON(sources config.SourceMap, discovery []config.DiscoveryEvent, violations []config.TrustViolation) error {
	ctx := make(map[string]string)
	ctx["user_config"] = appLoader.UserFile()

	cfgOut := make(map[string]configValueOutput)

	violationsByKey := make(map[string]config.ViolationReason, len(violations))
	for _, v := range violations {
		violationsByKey[v.Key] = v.Reason
	}

	for k, entry := range sources {
		val := entry.Value
		if config.IsSensitiveKey(k) {
			val = "********"
		}

		protection := ""
		if config.IsImmutable(k) {
			protection = "immutable"
		} else if reason, ok := violationsByKey[k]; ok && reason == config.ViolationUntrustedProject {
			protection = "untrusted"
		}

		cfgOut[k] = configValueOutput{
			Value:      val,
			Source:     entry.Source.String(),
			File:       entry.File,
			Protection: protection,
		}
	}

	output := debugConfigOutput{
		Context:    ctx,
		Discovery:  discovery,
		Config:     cfgOut,
		Violations: violations,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
