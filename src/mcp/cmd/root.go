package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "quint-code",
	Short: "First Principles Framework (FPF) for AI-assisted engineering",
	Long: `Quint Code is a structured reasoning engine that implements the
First Principles Framework (FPF) for AI-assisted engineering.

It provides tools for hypothesis generation, verification, validation,
and decision-making with full audit trails.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
