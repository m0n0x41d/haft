package specflow

import (
	"slices"
	"testing"
)

func TestCanonicalDraftContractPublishesProfileIndependentGrammar(t *testing.T) {
	contract := CanonicalDraftContract()

	if contract.ContractVersion != DraftContractVersion ||
		contract.Authority != DraftContractAuthority {
		t.Fatalf("draft contract identity = %#v", contract)
	}
	if len(contract.Phases) != len(PhaseRegistry()) {
		t.Fatalf(
			"draft contract phases = %d, want %d",
			len(contract.Phases),
			len(PhaseRegistry()),
		)
	}
	target, found := draftPhaseContractByID(
		contract.Phases,
		PhaseTargetBoundaryDraft,
	)
	if !found || target.SectionKind != "target.boundary" ||
		!slices.Contains(target.ExpectedFields, "target_refs") ||
		!slices.Contains(
			target.Checks,
			"require_boundary_perspectives:min=4",
		) {
		t.Fatalf("target boundary contract = %#v", target)
	}
	if !slices.Contains(contract.SpecSection.StatementTypes, "definition") ||
		!slices.Contains(contract.SpecSection.ClaimLayers, "object") ||
		!slices.Contains(contract.SpecSection.Statuses, "draft") {
		t.Fatalf("spec-section enums = %#v", contract.SpecSection)
	}
	if contract.ValidationCall.Tool != "haft_query" ||
		contract.ValidationCall.Arguments["action"] != "spec_validate" {
		t.Fatalf("validation call = %#v", contract.ValidationCall)
	}
}

func draftPhaseContractByID(
	phases []DraftPhaseContract,
	id PhaseID,
) (DraftPhaseContract, bool) {
	index := slices.IndexFunc(phases, func(phase DraftPhaseContract) bool {
		return phase.PhaseID == id
	})
	if index < 0 {
		return DraftPhaseContract{}, false
	}
	return phases[index], true
}
