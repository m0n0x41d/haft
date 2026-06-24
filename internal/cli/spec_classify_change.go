package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
)

func runSpecClassifyChange(cmd *cobra.Command, _ []string) error {
	input, err := specClassifyChangeInputFromFlags()
	if err != nil {
		return err
	}

	report, err := classifySpecCarrierChange(input)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if specClassifyChangeJSON {
		return writeSpecCarrierChangeJSON(output, report)
	}
	return writeSpecCarrierChangeText(output, report)
}

type specCarrierChangeInput struct {
	BeforePath string
	AfterPath  string
	SectionID  string
	Kind       string
	DryRun     bool
}

func specClassifyChangeInputFromFlags() (specCarrierChangeInput, error) {
	input := specCarrierChangeInput{
		BeforePath: strings.TrimSpace(specClassifyBefore),
		AfterPath:  strings.TrimSpace(specClassifyAfter),
		SectionID:  strings.TrimSpace(specClassifySection),
		Kind:       strings.TrimSpace(specClassifyKind),
	}
	if input.BeforePath == "" {
		return specCarrierChangeInput{}, fmt.Errorf("spec classify-change requires --before")
	}
	if input.AfterPath == "" {
		return specCarrierChangeInput{}, fmt.Errorf("spec classify-change requires --after")
	}
	if input.SectionID == "" {
		return specCarrierChangeInput{}, fmt.Errorf("spec classify-change requires --section")
	}
	return input, nil
}

func classifySpecCarrierChange(input specCarrierChangeInput) (project.SpecCarrierChangeReport, error) {
	before, err := loadSpecCarrierChangeSection(input.BeforePath, input.Kind, input.SectionID)
	if err != nil {
		return project.SpecCarrierChangeReport{}, err
	}
	after, err := loadSpecCarrierChangeSection(input.AfterPath, input.Kind, input.SectionID)
	if err != nil {
		return project.SpecCarrierChangeReport{}, err
	}
	return project.ClassifySpecSectionCarrierChange(before, after), nil
}

func loadSpecCarrierChangeSection(path string, kind string, sectionID string) (project.SpecSection, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return project.SpecSection{}, fmt.Errorf("read %s: %w", path, err)
	}

	document := project.SpecDocumentInput{
		Path:    filepath.ToSlash(path),
		Kind:    specCarrierChangeKind(path, kind),
		Content: string(content),
	}
	sections := project.SpecSectionsFromDocuments([]project.SpecDocumentInput{document})
	for _, section := range sections {
		if section.ID == sectionID {
			return section, nil
		}
	}
	return project.SpecSection{}, fmt.Errorf("section %q not found in %s", sectionID, path)
}

func specCarrierChangeKind(path string, override string) string {
	trimmed := strings.TrimSpace(override)
	if trimmed != "" {
		return trimmed
	}
	switch filepath.Base(path) {
	case "target-system.md":
		return "target-system"
	case "enabling-system.md":
		return "enabling-system"
	case "term-map.md":
		return "term-map"
	default:
		return "unknown"
	}
}

func writeSpecCarrierChangeJSON(writer io.Writer, report project.SpecCarrierChangeReport) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func writeSpecCarrierChangeText(writer io.Writer, report project.SpecCarrierChangeReport) error {
	lines := []string{
		fmt.Sprintf("spec carrier change: %s", report.Kind),
		fmt.Sprintf("section_id: %s", report.SectionID),
		fmt.Sprintf("import_posture: %s", report.ImportPosture),
		fmt.Sprintf("source_of_truth: %s", report.SourceOfTruth),
		fmt.Sprintf("apply_boundary: %s", report.ApplyBoundary),
	}
	lines = appendOptionalFieldLine(lines, "scalar_fields", report.ScalarFields)
	lines = appendOptionalFieldLine(lines, "relationship_fields", report.RelationshipFields)
	lines = appendOptionalFieldLine(lines, "carrier_only_fields", report.CarrierOnlyFields)
	lines = appendOptionalFieldLine(lines, "high_risk_fields", report.HighRiskFields)
	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}
	return nil
}

func appendOptionalFieldLine(lines []string, label string, values []string) []string {
	if len(values) == 0 {
		return lines
	}
	return append(lines, fmt.Sprintf("%s: %s", label, strings.Join(values, ", ")))
}
