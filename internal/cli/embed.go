package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/embedding"
)

var (
	embedStatusJSON      bool
	embedStatusFootprint bool
)

var embedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Inspect local embedding sidecar diagnostics",
}

var embedStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show live haft-embed sidecar processes and socket contracts",
	Long: `Show read-only diagnostics for local haft-embed sidecars.

The command does not start the embedding model. It inspects the shared socket
directory and current process table, then reports which sidecars are active,
stale, stdio-only, or process-only.`,
	Args: cobra.NoArgs,
	RunE: runEmbedStatus,
}

func init() {
	embedStatusCmd.Flags().BoolVar(&embedStatusJSON, "json", false, "print structured JSON output")
	embedStatusCmd.Flags().BoolVar(&embedStatusFootprint, "footprint", false, "probe macOS physical footprint via vmmap")
	embedCmd.AddCommand(embedStatusCmd)
	rootCmd.AddCommand(embedCmd)
}

func runEmbedStatus(cmd *cobra.Command, _ []string) error {
	report, err := embedding.LoadSidecarStatus(
		cmd.Context(),
		embedding.SidecarStatusOptions{IncludeFootprint: embedStatusFootprint},
	)
	if err != nil {
		return err
	}
	if embedStatusJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	return writeEmbedStatusSummary(cmd.OutOrStdout(), report)
}

func writeEmbedStatusSummary(w io.Writer, report embedding.SidecarStatusReport) error {
	builder := strings.Builder{}

	shared := "disabled"
	if report.SharedEnabled {
		shared = "enabled"
	}
	binary := "not found"
	if report.BinaryFound {
		binary = report.Binary
	}

	builder.WriteString(fmt.Sprintf(
		"haft embed status: shared=%s sidecars=%d socket_dir=%s\n",
		shared,
		len(report.Entries),
		report.SocketDir,
	))
	builder.WriteString(fmt.Sprintf("binary: %s\n", binary))

	if len(report.Entries) == 0 {
		builder.WriteString("sidecars: none observed\n")
	}
	for _, entry := range report.Entries {
		builder.WriteString(embedStatusEntryLine(entry))
		builder.WriteString(embedStatusEntryDetailLines(entry))
	}

	if len(report.Warnings) > 0 {
		builder.WriteString("warnings:\n")
		for _, warning := range report.Warnings {
			builder.WriteString(fmt.Sprintf("  - %s\n", warning))
		}
	}
	builder.WriteString("memory_note: rss/vsz come from ps; use --footprint on macOS for physical footprint.\n")

	_, err := io.WriteString(w, builder.String())
	return err
}

func embedStatusEntryLine(entry embedding.SidecarStatusEntry) string {
	pid := "none"
	if entry.PID > 0 {
		pid = fmt.Sprintf("%d", entry.PID)
	}
	dim := entry.DimLabel
	if dim == "" {
		dim = "native"
	}
	model := entry.Model
	if model == "" {
		model = "unknown"
	}
	idle := "unknown"
	if entry.IdleTimeoutSecs > 0 {
		idle = fmt.Sprintf("%ds", entry.IdleTimeoutSecs)
	}

	return fmt.Sprintf(
		"- %s state=%s pid=%s ppid=%s model=%s dim=%s rss=%s vsz=%s idle=%s\n",
		entry.Key,
		entry.State,
		pid,
		embedStatusInt(entry.PPID),
		model,
		dim,
		embedStatusMemory(entry.RSSKB),
		embedStatusMemory(entry.VSZKB),
		idle,
	)
}

func embedStatusEntryDetailLines(entry embedding.SidecarStatusEntry) string {
	builder := strings.Builder{}
	if entry.FootprintMB > 0 {
		builder.WriteString(fmt.Sprintf("  footprint: %.1fMB\n", entry.FootprintMB))
	}
	if entry.FootprintError != "" {
		builder.WriteString(fmt.Sprintf("  footprint_error: %s\n", entry.FootprintError))
	}
	if entry.SocketPath != "" {
		builder.WriteString(fmt.Sprintf(
			"  socket: %s exists=%t\n",
			entry.SocketPath,
			entry.SocketExists,
		))
	}
	if entry.LockPath != "" {
		builder.WriteString(fmt.Sprintf(
			"  lock: %s exists=%t\n",
			entry.LockPath,
			entry.LockExists,
		))
	}
	if entry.CacheDir != "" {
		builder.WriteString(fmt.Sprintf("  cache: %s\n", entry.CacheDir))
	}
	if entry.Command != "" {
		builder.WriteString(fmt.Sprintf("  command: %s\n", entry.Command))
	}
	return builder.String()
}

func embedStatusInt(value int) string {
	if value == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", value)
}

func embedStatusMemory(kb int64) string {
	if kb <= 0 {
		return "unknown"
	}
	mb := float64(kb) / 1024
	return fmt.Sprintf("%.1fMB", mb)
}
