package artifact

import (
	"fmt"
	"strings"
)

func appendSpecFitSection(body *strings.Builder, record *SpecFitRecord) {
	if record == nil {
		return
	}
	body.WriteString("\n## Spec Fit (Advisory)\n\n")
	body.WriteString(fmt.Sprintf("State: %s\n\n", record.State))
	if len(record.CandidateSectionRefs) > 0 {
		body.WriteString(fmt.Sprintf("Candidate SpecSections: %s\n\n", strings.Join(record.CandidateSectionRefs, ", ")))
	}
	if len(record.ConflictRefs) > 0 {
		body.WriteString(fmt.Sprintf("Conflict SpecSections: %s\n\n", strings.Join(record.ConflictRefs, ", ")))
	}
	if record.NextExpectedAction != "" {
		body.WriteString(fmt.Sprintf("Next expected action: %s\n\n", record.NextExpectedAction))
	}
	if len(record.VariantSpecFit) == 0 {
		return
	}
	body.WriteString("| Variant | State | SpecSections | Next action |\n")
	body.WriteString("|---------|-------|--------------|-------------|\n")
	for _, item := range record.VariantSpecFit {
		body.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %s |\n",
			specFitCell(item.VariantRef),
			specFitCell(item.State),
			specFitCell(strings.Join(item.SectionRefs, ", ")),
			specFitCell(item.ExpectedAction),
		))
	}
	body.WriteString("\n")
}

func appendVariantSpecFitSection(
	body *strings.Builder,
	items []SpecFitVariantRecord,
) {
	if len(items) == 0 {
		return
	}
	body.WriteString("\n## Variant Spec Fit (Advisory)\n\n")
	body.WriteString("| Variant | State | SpecSections | Next action |\n")
	body.WriteString("|---------|-------|--------------|-------------|\n")
	for _, item := range items {
		body.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %s |\n",
			specFitCell(item.VariantRef),
			specFitCell(item.State),
			specFitCell(strings.Join(item.SectionRefs, ", ")),
			specFitCell(item.ExpectedAction),
		))
	}
	body.WriteString("\n")
}

func specFitCell(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return strings.ReplaceAll(trimmed, "|", "/")
}

func cloneSpecFitRecord(record *SpecFitRecord) *SpecFitRecord {
	if record == nil {
		return nil
	}
	out := *record
	out.CandidateSectionRefs = append([]string(nil), record.CandidateSectionRefs...)
	out.ConflictRefs = append([]string(nil), record.ConflictRefs...)
	out.VariantSpecFit = cloneSpecFitVariantRecords(record.VariantSpecFit)
	return &out
}

func cloneSpecFitVariantRecords(items []SpecFitVariantRecord) []SpecFitVariantRecord {
	out := make([]SpecFitVariantRecord, 0, len(items))
	for _, item := range items {
		cloned := item
		cloned.SectionRefs = append([]string(nil), item.SectionRefs...)
		cloned.ConflictRefs = append([]string(nil), item.ConflictRefs...)
		out = append(out, cloned)
	}
	return out
}
