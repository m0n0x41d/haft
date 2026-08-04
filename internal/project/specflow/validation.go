package specflow

import "github.com/m0n0x41d/haft/internal/project"

const (
	CarrierValidationKind      = "spec_carrier_validation"
	CarrierValidationAuthority = "read_only_carrier_validation"
	CarrierValidationSource    = "authored_carriers_without_profile_applicability_filter"
)

type CarrierValidationReport struct {
	SchemaVersion         int                                `json:"schema_version"`
	ValidationKind        string                             `json:"validation_kind"`
	Authority             string                             `json:"authority"`
	SourceBasis           string                             `json:"source_basis"`
	AuthorityBoundary     CarrierValidationAuthorityBoundary `json:"authority_boundary"`
	Summary               CarrierValidationSummary           `json:"summary"`
	Structural            project.SpecCheckReport            `json:"structural"`
	Semantic              ReviewPacket                       `json:"semantic"`
	LifecycleObservations []project.SpecCheckFinding         `json:"lifecycle_observations"`
}

type CarrierValidationAuthorityBoundary struct {
	Applicability   string `json:"applicability"`
	Activation      string `json:"activation"`
	Approval        string `json:"approval"`
	Evidence        string `json:"evidence"`
	StrongerUse     string `json:"stronger_use"`
	LifecycleEffect string `json:"lifecycle_effect"`
	CarrierMutation string `json:"carrier_mutation"`
}

type CarrierValidationSummary struct {
	Documents             int `json:"documents"`
	TotalSections         int `json:"total_sections"`
	DraftSections         int `json:"draft_sections"`
	ActiveSections        int `json:"active_sections"`
	CheckedSections       int `json:"checked_sections"`
	StructuralFindings    int `json:"structural_findings"`
	SemanticFindings      int `json:"semantic_findings"`
	LifecycleObservations int `json:"lifecycle_observations"`
}

func ValidateCarrierSpecificationSet(
	set project.ProjectSpecificationSet,
) CarrierValidationReport {
	structural := project.SpecCheckReportFromSpecificationSet(set)
	structuralFindings, lifecycleObservations := splitCarrierValidationFindings(
		structural.Findings,
	)
	structural.Findings = structuralFindings
	structural.Summary.TotalFindings = len(structuralFindings)

	semanticSet := set
	semanticSet.Findings = append([]project.SpecCheckFinding(nil), structuralFindings...)
	semantic := ReviewDraftSpecificationSet(semanticSet)

	return CarrierValidationReport{
		SchemaVersion:     1,
		ValidationKind:    CarrierValidationKind,
		Authority:         CarrierValidationAuthority,
		SourceBasis:       CarrierValidationSource,
		AuthorityBoundary: carrierValidationAuthorityBoundary(),
		Summary: CarrierValidationSummary{
			Documents:             len(set.Documents),
			TotalSections:         len(set.Sections),
			DraftSections:         countReviewSectionsWithStatus(set.Sections, string(project.SpecSectionStateDraft), 0),
			ActiveSections:        semantic.Summary.ActiveSections,
			CheckedSections:       semantic.Summary.CheckedSections,
			StructuralFindings:    len(structural.Findings),
			SemanticFindings:      len(semantic.Findings),
			LifecycleObservations: len(lifecycleObservations),
		},
		Structural:            structural,
		Semantic:              semantic,
		LifecycleObservations: lifecycleObservations,
	}
}

func carrierValidationAuthorityBoundary() CarrierValidationAuthorityBoundary {
	return CarrierValidationAuthorityBoundary{
		Applicability:   "not_applicability_determination_or_admission",
		Activation:      "not_section_activation",
		Approval:        "not_approval_or_baseline",
		Evidence:        "not_evidence",
		StrongerUse:     "not_spec_use_admission",
		LifecycleEffect: "none_read_only",
		CarrierMutation: "none_read_only",
	}
}

func splitCarrierValidationFindings(
	findings []project.SpecCheckFinding,
) ([]project.SpecCheckFinding, []project.SpecCheckFinding) {
	structural := make([]project.SpecCheckFinding, 0, len(findings))
	lifecycle := make([]project.SpecCheckFinding, 0)
	for _, finding := range findings {
		if finding.Code == "spec_carrier_no_active_sections" {
			lifecycle = append(lifecycle, finding)
			continue
		}
		structural = append(structural, finding)
	}
	return structural, lifecycle
}
