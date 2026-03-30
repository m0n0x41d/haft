package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "haft [goal]",
	Short: "Engineering agent with FPF reasoning discipline",
	Long: `Haft — an engineering agent that thinks before it acts.

Run bare to launch the interactive agent. Use subcommands for specific tasks.

Examples:
  haft                              # launch interactive agent
  haft "fix the failing tests"      # agent with initial goal
  haft init                         # initialize project
  haft serve                        # start MCP server for other agents`,
	Version: Version,
	Args:    cobra.ArbitraryArgs,
	RunE:    runAgent, // bare haft = launch agent
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("haft %s\n", Version)
		fmt.Printf("  commit:  %s\n", Commit)
		fmt.Printf("  built:   %s\n", BuildDate)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(versionCmd)
	rootCmd.SetVersionTemplate("haft {{.Version}}\n")
}
