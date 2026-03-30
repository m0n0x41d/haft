package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/provider"
)

var modelsCmd = &cobra.Command{
	Use:   "models [filter]",
	Short: "List known LLM models",
	Long: `List all known LLM providers and models from the model registry.

The registry is loaded from catwalk.charm.sh with disk cache fallback
and embedded defaults compiled into the binary.

Examples:
  haft models
  haft models gpt
  haft models claude`,
	Args: cobra.MaximumNArgs(1),
	RunE: runModels,
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}

func runModels(cmd *cobra.Command, args []string) error {
	reg := provider.DefaultRegistry()

	filter := ""
	if len(args) > 0 {
		filter = args[0]
	}

	out := reg.FormatModelList(filter)
	if out == "" {
		if filter != "" {
			return fmt.Errorf("no models matching %q", filter)
		}
		return fmt.Errorf("no models in registry")
	}

	fmt.Print(out)
	return nil
}
