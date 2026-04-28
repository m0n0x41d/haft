package specflow

import (
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestSectionStalenessFindings_ActiveSectionPastValidUntilFlagged(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	section := activeEnvironmentSection()
	section.ValidUntil = "2026-04-25"

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	}, now)

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeSpecSectionStale {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeSpecSectionStale)
	}
	if findings[0].SectionID != section.ID {
		t.Fatalf("section_id = %q, want %q", findings[0].SectionID, section.ID)
	}
}

func TestSectionStalenessFindings_FreshSectionPasses(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	section := activeEnvironmentSection()
	section.ValidUntil = "2027-01-01"

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	}, now)

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 for fresh section", len(findings))
	}
}

func TestSectionStalenessFindings_DraftSectionSkipped(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	section := activeEnvironmentSection()
	section.Status = string(project.SpecSectionStateDraft)
	section.ValidUntil = "2026-01-01"

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	}, now)

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 for draft section even when valid_until is past", len(findings))
	}
}

func TestSectionStalenessFindings_DeprecatedSectionSkipped(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	section := activeEnvironmentSection()
	section.Status = string(project.SpecSectionStateDeprecated)
	section.ValidUntil = "2026-01-01"

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	}, now)

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 for deprecated section", len(findings))
	}
}

func TestSectionStalenessFindings_EmptyValidUntilSkipped(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	section := activeEnvironmentSection()
	section.ValidUntil = ""

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	}, now)

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 (structural check owns missing valid_until)", len(findings))
	}
}

func TestSectionStalenessFindings_MalformedValidUntilSkipped(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	section := activeEnvironmentSection()
	section.ValidUntil = "next tuesday"

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	}, now)

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 (RequireValidUntil owns malformed dates)", len(findings))
	}
}

func TestSectionStalenessFindings_RFC3339ValidUntilParsed(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	section := activeEnvironmentSection()
	section.ValidUntil = "2026-04-30T12:00:00Z"

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	}, now)

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
}

func TestSectionStalenessFindings_DefaultsToNowOnZeroTime(t *testing.T) {
	// Pass zero time -> helper substitutes time.Now(). Sections in 1970
	// will be stale; sections in 2099 will not.
	pastSection := activeEnvironmentSection()
	pastSection.ID = "tgt-past-1"
	pastSection.ValidUntil = "1970-01-01"

	futureSection := activeEnvironmentSection()
	futureSection.ID = "tgt-future-1"
	futureSection.ValidUntil = "2099-01-01"

	findings := SectionStalenessFindings(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{pastSection, futureSection},
	}, time.Time{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 (only past section flagged)", len(findings))
	}
	if findings[0].SectionID != "tgt-past-1" {
		t.Fatalf("section_id = %q, want tgt-past-1", findings[0].SectionID)
	}
}
