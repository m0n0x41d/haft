package specflow

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestValidateCarrierSpecificationSetReviewsDraftsWithoutLifecycleAdmission(
	t *testing.T,
) {
	section := project.SpecSection{
		ID:            "TS.boundary.001",
		Spec:          "target-system",
		DocumentKind:  "target-system",
		Kind:          "target.boundary",
		Title:         "Draft target boundary",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "human",
		Status:        string(project.SpecSectionStateDraft),
		Path:          ".haft/specs/target-system.md",
		Line:          7,
	}
	set := project.ProjectSpecificationSet{
		Documents: []project.SpecDocument{{
			Path:     section.Path,
			Kind:     project.SpecDocumentKindTargetSystem,
			Sections: []project.SpecSection{section},
		}},
		Sections: []project.SpecSection{section},
		Findings: []project.SpecCheckFinding{{
			Level:     "L1",
			Code:      "spec_carrier_no_active_sections",
			Path:      section.Path,
			FieldPath: "$.status",
			Message:   "draft sections remain review material",
		}},
	}

	report := ValidateCarrierSpecificationSet(set)

	if report.SourceBasis != CarrierValidationSource {
		t.Fatalf("source basis = %q", report.SourceBasis)
	}
	if report.Summary.DraftSections != 1 || report.Summary.CheckedSections != 1 {
		t.Fatalf("draft validation summary = %+v", report.Summary)
	}
	if report.Summary.StructuralFindings != 0 || report.Summary.LifecycleObservations != 1 {
		t.Fatalf("lifecycle observation was not separated from structural validation: %+v", report.Summary)
	}
	if len(report.Semantic.Sections) != 1 || report.Semantic.Sections[0].Status != "draft" {
		t.Fatalf("draft section was not semantically reviewed: %+v", report.Semantic.Sections)
	}
	if report.AuthorityBoundary.Applicability != "not_applicability_determination_or_admission" ||
		report.AuthorityBoundary.Approval != "not_approval_or_baseline" ||
		report.AuthorityBoundary.CarrierMutation != "none_read_only" {
		t.Fatalf("validation authority boundary = %+v", report.AuthorityBoundary)
	}
}

func TestValidateCarrierSpecificationSetPreservesStructuralFindings(
	t *testing.T,
) {
	set := project.ProjectSpecificationSet{
		Findings: []project.SpecCheckFinding{{
			Level:   "L1",
			Code:    "spec_section_missing_title",
			Path:    ".haft/specs/target-system.md",
			Message: "title is required",
		}},
	}

	report := ValidateCarrierSpecificationSet(set)

	if report.Summary.StructuralFindings != 1 {
		t.Fatalf("structural findings = %+v", report.Summary)
	}
	if report.Semantic.Summary.StructuralFindingsObserved != 1 {
		t.Fatalf("semantic review lost structural basis: %+v", report.Semantic.Summary)
	}
}

func TestValidateCarrierSpecificationSetDoesNotTreatDraftCarrierAsActive(
	t *testing.T,
) {
	section := project.SpecSection{
		ID:            "TS.placeholder.001",
		Spec:          "target-system",
		DocumentKind:  "target-system",
		Kind:          "environment-change",
		Title:         "Target system placeholder",
		StatementType: "explanation",
		ClaimLayer:    "carrier",
		Owner:         "human",
		Status:        string(project.SpecSectionStateDraft),
		Path:          ".haft/specs/target-system.md",
		Line:          15,
	}

	report := ValidateCarrierSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	})

	for _, finding := range report.Semantic.Findings {
		if finding.RuleID == "active_carrier_layer" {
			t.Fatalf("draft placeholder was reported as active: %+v", finding)
		}
	}
}
