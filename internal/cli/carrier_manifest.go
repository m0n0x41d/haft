package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
)

var carrierManifestJSON bool
var carrierCheckJSON bool

var carrierCmd = &cobra.Command{
	Use:   "carrier",
	Short: "Inspect carrier authority surfaces",
}

var carrierManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Show current/support/compat/provenance/archive carrier classes",
	Long: `Show the carrier authority manifest.

The manifest is a drill-down aid for agents and maintainers. It does not mutate
governance state and it is not included in default status output.`,
	RunE: runCarrierManifest,
}

var carrierCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Run fixed-point semio checks over current carrier wording",
	Long: `Run deterministic semio checks over current/support/compat carrier wording.

The check fails when dead runtime surfaces such as standalone agent, TUI, or
desktop wrappers are mentioned as current authority instead of being labeled as
dropped, archive, provenance, support, or not-current.`,
	RunE: runCarrierCheck,
}

func init() {
	carrierManifestCmd.Flags().BoolVar(&carrierManifestJSON, "json", false, "print the carrier manifest as JSON")
	carrierCheckCmd.Flags().BoolVar(&carrierCheckJSON, "json", false, "print semio check findings as JSON")
	carrierCmd.AddCommand(carrierManifestCmd)
	carrierCmd.AddCommand(carrierCheckCmd)
	rootCmd.AddCommand(carrierCmd)
}

func runCarrierManifest(cmd *cobra.Command, args []string) error {
	writer := cmd.OutOrStdout()
	manifest := project.DefaultCarrierAuthorityManifest()
	findings := project.ValidateCarrierAuthorityManifest(manifest)
	if len(findings) > 0 {
		return fmt.Errorf("carrier manifest invalid: %v", findings)
	}

	if carrierManifestJSON {
		data, err := project.CarrierAuthorityManifestJSON(manifest)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(writer, string(data))
		return nil
	}

	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(writer, format, args...)
		return err
	}

	if err := printf("Carrier Authority Manifest v%d\n", manifest.SchemaVersion); err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		err := writeCarrierManifestEntry(writer, entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func runCarrierCheck(cmd *cobra.Command, args []string) error {
	result, err := project.CheckCarrierSemioWithVirtualTexts(".", carrierCheckGeneratedSurfaces())
	if err != nil {
		return err
	}

	writer := cmd.OutOrStdout()
	if carrierCheckJSON {
		data, err := project.CarrierSemioCheckResultJSON(result)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(writer, string(data))
		return err
	}

	if len(result.Findings) == 0 {
		_, err := fmt.Fprintf(writer, "carrier semio check: clean (%d file(s), %d generated surface(s))\n", len(result.CheckedFiles), len(result.CheckedGeneratedSurfaces))
		return err
	}

	if _, err := fmt.Fprintf(writer, "carrier semio check: %d finding(s)\n", len(result.Findings)); err != nil {
		return err
	}
	for _, finding := range result.Findings {
		if _, err := fmt.Fprintf(
			writer,
			"- %s:%d %s — %s\n",
			finding.Path,
			finding.Line,
			finding.Term,
			finding.Diagnostic,
		); err != nil {
			return err
		}
	}
	return fmt.Errorf("carrier semio check found %d issue(s)", len(result.Findings))
}

func carrierCheckGeneratedSurfaces() []project.CarrierSemioVirtualText {
	capabilities := haftInterfaceCatalog()
	surfaces := make([]project.CarrierSemioVirtualText, 0, len(capabilities))
	for _, capability := range capabilities {
		surfaces = append(surfaces, project.CarrierSemioVirtualText{
			Path:    "generated/interface/" + capability.ID,
			Content: carrierCheckGeneratedSurfaceText(capability),
		})
	}
	return surfaces
}

func carrierCheckGeneratedSurfaceText(capability interfaceCapability) string {
	var builder strings.Builder
	builder.WriteString(capability.ID)
	builder.WriteByte('\n')
	builder.WriteString(capability.Purpose)
	builder.WriteByte('\n')
	builder.WriteString(capability.CurrentExecution.MCPCall)
	builder.WriteByte('\n')
	builder.WriteString(capability.CurrentExecution.CLICommand)
	builder.WriteByte('\n')
	for _, shape := range capability.InputContract.FieldShapes {
		builder.WriteString(shape.Field)
		builder.WriteByte('\n')
		builder.WriteString(shape.Shape)
		builder.WriteByte('\n')
		builder.WriteString(shape.Note)
		builder.WriteByte('\n')
	}
	for _, note := range capability.InputContract.Notes {
		builder.WriteString(note)
		builder.WriteByte('\n')
	}
	for _, output := range capability.OutputVolume {
		builder.WriteString(output)
		builder.WriteByte('\n')
	}
	for _, invariant := range capability.Invariants {
		builder.WriteString(invariant)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func writeCarrierManifestEntry(writer io.Writer, entry project.CarrierManifestEntry) error {
	current := "historical"
	if entry.Current {
		current = "current"
	}
	if _, err := fmt.Fprintf(writer, "- %s [%s/%s] %s\n", entry.ID, entry.AuthorityClass, current, entry.PathPattern); err != nil {
		return err
	}
	if entry.Normativity != "" {
		if _, err := fmt.Fprintf(writer, "  normativity: %s\n", entry.Normativity); err != nil {
			return err
		}
	}
	if entry.DeadSurfacePolicy != "" {
		if _, err := fmt.Fprintf(writer, "  dead_surface_policy: %s\n", entry.DeadSurfacePolicy); err != nil {
			return err
		}
	}
	return nil
}
