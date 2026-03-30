package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/setup"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure models and authentication",
	Long: `Interactive setup for Haft.

Pick models, enter credentials for one or more providers.
Runs automatically on first launch if not configured.

Examples:
  haft setup`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(_ *cobra.Command, _ []string) error {
	result, err := setup.Run()
	if err != nil {
		return err
	}

	fmt.Printf("\nDefault model: %s\n", result.Config.Model)
	fmt.Printf("Providers: %s\n", strings.Join(result.Config.ConfiguredProviders(), ", "))
	fmt.Printf("Config saved to %s\n", config.ConfigPath())
	return nil
}

// ensureConfigured checks if haft is configured, runs setup if not.
// Returns the loaded config. Used by agent command on startup.
func ensureConfigured() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if cfg.IsConfigured() {
		return cfg, nil
	}

	// Not configured — run interactive setup
	fmt.Println("First run — let's set up Haft.")
	result, err := setup.Run()
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}

	return result.Config, nil
}
