package cli

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "haft",
	Short: "FPF governance substrate — kernel + CLI + MCP server + skills",
	Long: `Haft — a governance substrate that makes a repository harnessable for
principal-led FPF engineering work by turning problem frames, comparisons,
decisions, commissions, and evidence into auditable artifacts.

Haft is consumed via three surfaces sharing one artifact graph:
  - Skills + slash commands in your AI coding agent (Claude Code, Codex, etc.)
  - This CLI for direct manual access without an LLM
  - MCP server for programmatic access from any LLM agent

Standalone interactive agent and TUI surfaces were dropped in v8.0 per
the governance substrate pivot — operate haft through your existing
coding agent (recommended) or via subcommands below.

Examples:
  haft init                         # install skills + MCP config, set up project
  haft serve                        # start the MCP server for other agents
  haft fpf search "B.5.2"           # search the FPF specification
  haft doctor                       # check installation health`,
	Version: Version,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		info := effectiveBuildInfo(readVCSBuildSettings())
		fmt.Printf("haft %s\n", Version)
		fmt.Printf("  commit:  %s\n", info.Commit)
		fmt.Printf("  built:   %s\n", info.BuildDate)
		if info.SourceTime != "" {
			fmt.Printf("  source:  %s\n", info.SourceTime)
		}
		if info.Modified {
			fmt.Printf("  modified: true\n")
		}
	},
}

type buildInfo struct {
	Commit     string
	BuildDate  string
	SourceTime string
	Modified   bool
}

func effectiveBuildInfo(settings map[string]string) buildInfo {
	info := buildInfo{
		Commit:    Commit,
		BuildDate: BuildDate,
	}
	if info.Commit == "" || info.Commit == "none" {
		if revision := settings["vcs.revision"]; revision != "" {
			info.Commit = revision
		}
	}
	if info.Commit == "" {
		info.Commit = "none"
	}
	if info.BuildDate == "" {
		info.BuildDate = "unknown"
	}
	info.SourceTime = settings["vcs.time"]
	info.Modified = settings["vcs.modified"] == "true"
	return info
}

func readVCSBuildSettings() map[string]string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	return settings
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
