package specflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	codeSpecSectionNeedsBaseline = "spec_section_needs_baseline"
	codeSpecSectionDrifted       = "spec_section_drifted"
	codeSpecSectionStale         = "spec_section_stale"
)

// SectionBaselineFindings returns drift / missing-baseline findings for
// every active section in the spec set against the given store. Draft,
// deprecated, and superseded sections are skipped — baseline only
// applies to sections the operator has approved.
//
// Returns an empty slice when every active section has a current,
// matching baseline.
func SectionBaselineFindings(set project.ProjectSpecificationSet, store BaselineStore, projectID string) []project.SpecCheckFinding {
	if store == nil {
		return nil
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil
	}

	var findings []project.SpecCheckFinding
	for _, section := range set.Sections {
		if !sectionIsActive(section) {
			continue
		}

		baseline, err := store.Get(projectID, section.ID)
		currentHash := HashSection(section)

		if errors.Is(err, BaselineNotFound) {
			findings = append(findings, project.SpecCheckFinding{
				Level:      FindingLevelError,
				Code:       codeSpecSectionNeedsBaseline,
				Path:       section.Path,
				Line:       section.Line,
				SectionID:  section.ID,
				Message:    fmt.Sprintf("section %q is active but has no baseline; the operator has not yet approved it through the onboarding method", section.ID),
				NextAction: fmt.Sprintf("haft_spec_section(action=\"approve\", section_id=%q) to record a baseline", section.ID),
			})
			continue
		}
		if err != nil {
			findings = append(findings, project.SpecCheckFinding{
				Level:     FindingLevelError,
				Code:      codeSpecSectionNeedsBaseline,
				Path:      section.Path,
				Line:      section.Line,
				SectionID: section.ID,
				Message:   fmt.Sprintf("section %q baseline lookup failed: %v", section.ID, err),
			})
			continue
		}

		if baseline.Hash == currentHash {
			continue
		}

		findings = append(findings, project.SpecCheckFinding{
			Level:      FindingLevelError,
			Code:       codeSpecSectionDrifted,
			Path:       section.Path,
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q drifted from baseline (baseline=%s current=%s, captured_at=%s)", section.ID, shortHash(baseline.Hash), shortHash(currentHash), baseline.CapturedAt.Format("2006-01-02")),
			NextAction: fmt.Sprintf("triage drift: rebaseline if intentional evolution, reopen if review needed, or rollback the carrier edit on %q", section.ID),
		})
	}

	return findings
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
