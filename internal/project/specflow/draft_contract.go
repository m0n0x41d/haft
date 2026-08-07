package specflow

import (
	"github.com/m0n0x41d/haft/internal/project"
)

const (
	DraftContractVersion   = "haft.spec-draft-contract/v1"
	DraftContractAuthority = "read_only_design_time_contract_not_applicability_approval_or_evidence"
)

// DraftContract publishes the design-time grammar that an agent needs before
// a project profile can establish lifecycle applicability. It contains no
// project state and cannot make a SpecSection applicable, active, or approved.
type DraftContract struct {
	ContractVersion     string                   `json:"contract_version"`
	Authority           string                   `json:"authority"`
	ApplicabilityEffect string                   `json:"applicability_effect"`
	LifecycleEffect     string                   `json:"lifecycle_effect"`
	Phases              []DraftPhaseContract     `json:"phases"`
	SpecSection         SpecSectionDraftContract `json:"spec_section"`
	TermMap             TermMapDraftContract     `json:"term_map"`
	ValidationCall      DraftToolCall            `json:"validation_call"`
}

type DraftPhaseContract struct {
	PhaseID         PhaseID                  `json:"phase_id"`
	Required        bool                     `json:"required"`
	DependsOn       []PhaseID                `json:"depends_on"`
	DocumentKind    project.SpecDocumentKind `json:"document_kind"`
	SectionKind     string                   `json:"section_kind"`
	PromptForUser   string                   `json:"prompt_for_user"`
	ContextForAgent string                   `json:"context_for_agent"`
	ExpectedFields  []string                 `json:"expected_fields"`
	Checks          []string                 `json:"checks"`
}

type SpecSectionDraftContract struct {
	FenceInfo             string                `json:"fence_info"`
	RequiredFields        []string              `json:"required_fields"`
	OptionalFields        []string              `json:"optional_fields"`
	StatementTypes        []string              `json:"statement_types"`
	ClaimLayers           []string              `json:"claim_layers"`
	Owners                []string              `json:"owners"`
	Statuses              []string              `json:"statuses"`
	SystemFrames          []string              `json:"system_frames"`
	EvidenceRequiredShape DraftListItemContract `json:"evidence_required_shape"`
	ClaimShape            DraftClaimContract    `json:"claim_shape"`
}

type DraftListItemContract struct {
	AcceptedForms []string `json:"accepted_forms"`
	ObjectFields  []string `json:"object_fields,omitempty"`
}

type DraftClaimContract struct {
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields"`
	ClassReading   string   `json:"class_reading"`
}

type TermMapDraftContract struct {
	FenceInfo      string   `json:"fence_info"`
	ContainerField string   `json:"container_field"`
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields"`
	Compatibility  string   `json:"compatibility"`
}

type DraftToolCall struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
}

// CanonicalDraftContract returns only shipped, profile-independent product
// knowledge. Project evidence and current lifecycle state belong to other
// surfaces.
func CanonicalDraftContract() DraftContract {
	phases := PhaseRegistry()
	phaseContracts := make([]DraftPhaseContract, len(phases))
	fillDraftPhaseContracts(phases, phaseContracts, 0)

	validationArguments := map[string]interface{}{
		"action": "spec_validate",
	}
	validationCall := DraftToolCall{
		Tool:      "haft_query",
		Arguments: validationArguments,
	}

	return DraftContract{
		ContractVersion:     DraftContractVersion,
		Authority:           DraftContractAuthority,
		ApplicabilityEffect: "none_contract_does_not_establish_profile_applicability",
		LifecycleEffect:     "none_contract_does_not_activate_approve_rebaseline_or_reopen",
		Phases:              phaseContracts,
		SpecSection:         canonicalSpecSectionDraftContract(),
		TermMap:             canonicalTermMapDraftContract(),
		ValidationCall:      validationCall,
	}
}

func fillDraftPhaseContracts(
	phases []Phase,
	result []DraftPhaseContract,
	index int,
) {
	if index == len(phases) {
		return
	}
	phase := phases[index]
	result[index] = DraftPhaseContract{
		PhaseID:         phase.ID,
		Required:        phase.Required,
		DependsOn:       append([]PhaseID{}, phase.DependsOn...),
		DocumentKind:    phase.DocumentKind,
		SectionKind:     phase.SectionKind,
		PromptForUser:   phase.PromptForUser,
		ContextForAgent: phase.ContextForAgent,
		ExpectedFields:  append([]string{}, phase.ExpectedFields...),
		Checks:          phaseCheckNames(phase),
	}
	fillDraftPhaseContracts(phases, result, index+1)
}

func canonicalSpecSectionDraftContract() SpecSectionDraftContract {
	evidenceShape := DraftListItemContract{
		AcceptedForms: []string{
			"non_empty_string",
			"object",
		},
		ObjectFields: []string{
			"kind",
			"description",
		},
	}
	claimShape := DraftClaimContract{
		RequiredFields: []string{
			"id",
			"class",
		},
		OptionalFields: []string{
			"statement",
			"claim",
			"scope",
			"support_refs",
			"evidence_refs",
			"valid_until",
			"governing_pattern_refs",
		},
		ClassReading: "L/A/D/E classes are resolved by advisory semantic review; mixed or unknown classes abstain from stronger use.",
	}
	return SpecSectionDraftContract{
		FenceInfo:      "yaml spec-section",
		RequiredFields: []string{"id", "kind", "statement_type", "claim_layer", "owner", "status"},
		OptionalFields: []string{
			"spec",
			"title",
			"system_frame",
			"valid_until",
			"terms",
			"depends_on",
			"target_refs",
			"evidence_required",
			"claims",
			"carrier_claim_allowed",
		},
		StatementTypes:        append([]string{}, project.SpecSectionValidStatementTypes...),
		ClaimLayers:           append([]string{}, project.SpecSectionValidClaimLayers...),
		Owners:                append([]string{}, project.SpecSectionValidOwners...),
		Statuses:              append([]string{}, project.SpecSectionValidStatuses...),
		SystemFrames:          []string{"target_system", "software_system", "carrier", "sidekick"},
		EvidenceRequiredShape: evidenceShape,
		ClaimShape:            claimShape,
	}
}

func canonicalTermMapDraftContract() TermMapDraftContract {
	return TermMapDraftContract{
		FenceInfo:      "yaml term-map",
		ContainerField: "entries",
		RequiredFields: []string{"term", "category", "definition"},
		OptionalFields: []string{"not", "aliases", "owners"},
		Compatibility:  "legacy domain is accepted only as a compatibility alias for category and must not conflict with it",
	}
}
