package specflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

// SectionStalenessFindings returns spec_section_stale findings for every
// active SpecSection whose valid_until parses as RFC3339 or YYYY-MM-DD
// and is strictly before now. Draft / deprecated / superseded /
// malformed sections are skipped — staleness only applies to operator-
// approved active claims.
//
// Sections with empty or unparseable valid_until are NOT flagged here;
// the structural speccheck pipeline catches missing valid_until per
// phase Checks. Skipping malformed values from this helper avoids
// double-reporting the same problem.
//
// The helper is the canonical source consumed by both `haft spec check`
// (CLI) and `haft_query(action="check")` (MCP). Per dec-20260428-spec-
// enforcement-hardening-219a58b5: one helper, no parallel
// reimplementations.
func SectionStalenessFindings(set project.ProjectSpecificationSet, now time.Time) []project.SpecCheckFinding {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var findings []project.SpecCheckFinding
	for _, section := range set.Sections {
		if !sectionIsActive(section) {
			continue
		}

		raw := strings.TrimSpace(section.ValidUntil)
		if raw == "" {
			continue
		}

		validUntil, ok := parseSectionValidUntil(raw)
		if !ok {
			continue
		}

		if !validUntil.Before(now) {
			continue
		}

		daysOverdue := int(now.Sub(validUntil).Hours() / 24)
		findings = append(findings, project.SpecCheckFinding{
			Level:      FindingLevelError,
			Code:       codeSpecSectionStale,
			Path:       section.Path,
			FieldPath:  "valid_until",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q is active but valid_until %s expired %d day(s) ago", section.ID, raw, daysOverdue),
			NextAction: fmt.Sprintf("triage staleness on %q: rebaseline if the claim is still current (extend valid_until in the carrier and run haft_spec_section action=rebaseline), reopen if the claim needs review, or deprecate the section", section.ID),
		})
	}

	return findings
}

func parseSectionValidUntil(raw string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
