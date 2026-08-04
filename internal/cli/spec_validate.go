package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

var (
	specValidateJSON bool
	specValidateExit = os.Exit
)

var specValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate draft and active spec carriers without lifecycle admission",
	Long: `Run carrier-first structural checks and advisory semantic review over
draft and active SpecSections.

Validation reads authored target-system, software-system, and term-map carriers
without filtering them through canonical profile applicability. It does not
determine or admit applicability, activate or approve sections, create evidence,
authorize stronger use, mutate carriers, or cross any lifecycle gate.

Lifecycle observations such as "no active sections" remain visible but are
separate from structural validation findings and do not prevent semantic review
of draft sections. Semantic findings remain advisory.`,
	Args: cobra.NoArgs,
	RunE: runSpecValidate,
}

func init() {
	specValidateCmd.Flags().BoolVar(
		&specValidateJSON,
		"json",
		false,
		"print structured JSON output",
	)
	specCmd.AddCommand(specValidateCmd)
}

func runSpecValidate(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	report, err := buildSpecValidationReport(projectRoot)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if specValidateJSON {
		err = writeSpecValidationJSON(output, report)
	} else {
		err = writeSpecValidationSummary(output, report)
	}
	if err != nil {
		return err
	}
	if report.Summary.StructuralFindings > 0 {
		specValidateExit(1)
	}
	return nil
}

func buildSpecValidationReport(
	projectRoot string,
) (specflow.CarrierValidationReport, error) {
	set, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return specflow.CarrierValidationReport{}, fmt.Errorf(
			"load authored spec carriers for validation: %w",
			err,
		)
	}
	return specflow.ValidateCarrierSpecificationSet(set), nil
}

func writeSpecValidationJSON(
	w io.Writer,
	report specflow.CarrierValidationReport,
) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func writeSpecValidationSummary(
	w io.Writer,
	report specflow.CarrierValidationReport,
) error {
	builder := strings.Builder{}
	builder.WriteString("haft spec validate: carrier-first read-only validation\n")
	builder.WriteString(fmt.Sprintf("source_basis: %s\n", report.SourceBasis))
	builder.WriteString(fmt.Sprintf(
		"sections: total=%d draft=%d active=%d checked=%d\n",
		report.Summary.TotalSections,
		report.Summary.DraftSections,
		report.Summary.ActiveSections,
		report.Summary.CheckedSections,
	))
	builder.WriteString(fmt.Sprintf(
		"findings: structural=%d semantic=%d lifecycle_observations=%d\n",
		report.Summary.StructuralFindings,
		report.Summary.SemanticFindings,
		report.Summary.LifecycleObservations,
	))
	builder.WriteString(fmt.Sprintf(
		"authority: %s; applicability=%s activation=%s approval=%s evidence=%s stronger_use=%s lifecycle_effect=%s carrier_mutation=%s\n",
		report.Authority,
		report.AuthorityBoundary.Applicability,
		report.AuthorityBoundary.Activation,
		report.AuthorityBoundary.Approval,
		report.AuthorityBoundary.Evidence,
		report.AuthorityBoundary.StrongerUse,
		report.AuthorityBoundary.LifecycleEffect,
		report.AuthorityBoundary.CarrierMutation,
	))

	appendSpecValidationCheckFindings(
		&builder,
		"Structural findings",
		report.Structural.Findings,
	)
	appendSpecValidationCheckFindings(
		&builder,
		"Lifecycle observations",
		report.LifecycleObservations,
	)
	if len(report.Semantic.Findings) > 0 {
		builder.WriteString("\nAdvisory semantic findings:\n")
	}
	for _, finding := range report.Semantic.Findings {
		builder.WriteString(formatSpecReviewFinding(finding))
	}

	_, err := io.WriteString(w, builder.String())
	return err
}

func appendSpecValidationCheckFindings(
	builder *strings.Builder,
	title string,
	findings []project.SpecCheckFinding,
) {
	if len(findings) == 0 {
		return
	}
	builder.WriteString("\n" + title + ":\n")
	for _, finding := range findings {
		builder.WriteString(formatSpecCheckFinding(finding))
	}
}
