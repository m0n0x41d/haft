package specflow

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestSpecBindingPreflightAllowsNoSpecProjects(t *testing.T) {
	result := BuildSpecBindingPreflight(
		project.ProjectSpecificationSet{},
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle: "Local tactical cleanup",
				Mode:          SpecBindingDecisionModeStandard,
			},
		},
	)

	if result.State != SpecBindingStateNoSpecs {
		t.Fatalf("state = %q, want %q", result.State, SpecBindingStateNoSpecs)
	}
	if !containsSpecBindingTestString(result.AllowedNextActions, "create_decision") {
		t.Fatalf("allowed_next_actions = %#v, want create_decision", result.AllowedNextActions)
	}
	if result.StatusDebt.Severity != SpecBindingDebtLow {
		t.Fatalf("status_debt = %#v, want low debt nudge", result.StatusDebt)
	}
}

func TestSpecBindingPreflightAllowsNoActiveSections(t *testing.T) {
	result := BuildSpecBindingPreflight(
		project.ProjectSpecificationSet{
			Sections: []project.SpecSection{specBindingTestSection("TS.draft.001", project.SpecSectionStateDraft)},
		},
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle: "Draft-only project decision",
			},
		},
	)

	if result.State != SpecBindingStateNoActiveSections {
		t.Fatalf("state = %q, want %q", result.State, SpecBindingStateNoActiveSections)
	}
	if result.OperatorActionRequired != SpecBindingOperatorActionNone {
		t.Fatalf("operator_action_required = %q", result.OperatorActionRequired)
	}
}

func TestSpecBindingPreflightValidatesProvidedRefs(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{specBindingTestSection("TS.boundary.001", project.SpecSectionStateActive)},
	}
	result := BuildSpecBindingPreflight(
		specSet,
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle: "Boundary decision",
				SectionRefs:   []string{"spec_section:TS.boundary.001"},
			},
		},
	)

	if result.State != SpecBindingStateProvidedRefsValid {
		t.Fatalf("state = %q, want %q", result.State, SpecBindingStateProvidedRefsValid)
	}
	if !containsSpecBindingTestString(result.SelectedSectionRefs, "TS.boundary.001") {
		t.Fatalf("selected_section_refs = %#v", result.SelectedSectionRefs)
	}
}

func TestSpecBindingPreflightFailsClosedOnInvalidRefs(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			specBindingTestSection("TS.active.001", project.SpecSectionStateActive),
			specBindingTestSection("TS.draft.001", project.SpecSectionStateDraft),
		},
	}
	result := BuildSpecBindingPreflight(
		specSet,
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle: "Bad section refs",
				SectionRefs:   []string{"TS.missing.001", "TS.draft.001"},
			},
		},
	)

	if result.State != SpecBindingStateInvalidRefs {
		t.Fatalf("state = %q, want %q", result.State, SpecBindingStateInvalidRefs)
	}
	if len(result.InvalidRefs) != 2 {
		t.Fatalf("invalid_refs = %#v, want two invalid refs", result.InvalidRefs)
	}
	if result.BlockedNextActions[0] != "create_spec_bound_decision" {
		t.Fatalf("blocked_next_actions = %#v", result.BlockedNextActions)
	}
}

func TestSpecBindingPreflightBindsSingleHighConfidenceExistingSection(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			specBindingTestSectionWithTargets("TS.boundary.001", project.SpecSectionStateActive, []string{"symbol:internal/cli/spec.go::runSpecUse"}),
			specBindingTestSectionWithTargets("TS.other.001", project.SpecSectionStateActive, []string{"symbol:internal/cli/other.go::Run"}),
		},
	}
	result := BuildSpecBindingPreflight(
		specSet,
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle:        "Spec use boundary repair",
				DecisionSubjectRef:   "symbol:internal/cli/spec.go::runSpecUse",
				GovernanceTargetRefs: []string{"symbol:internal/cli/spec.go::runSpecUse"},
			},
		},
	)

	if result.State != SpecBindingStateBoundExisting {
		t.Fatalf("state = %q, want %q; candidates=%#v", result.State, SpecBindingStateBoundExisting, result.CandidateSectionRefs)
	}
	if len(result.SelectedSectionRefs) != 1 || result.SelectedSectionRefs[0] != "TS.boundary.001" {
		t.Fatalf("selected_section_refs = %#v", result.SelectedSectionRefs)
	}
}

func TestSpecBindingPreflightBlocksAmbiguousHighConfidenceMatches(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			specBindingTestSection("TS.left.001", project.SpecSectionStateActive),
			specBindingTestSection("TS.right.001", project.SpecSectionStateActive),
		},
	}
	result := BuildSpecBindingPreflight(
		specSet,
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle: "Decision touching TS.left.001 and TS.right.001",
			},
		},
	)

	if result.State != SpecBindingStateAmbiguous {
		t.Fatalf("state = %q, want %q; candidates=%#v", result.State, SpecBindingStateAmbiguous, result.CandidateSectionRefs)
	}
	if result.OperatorActionRequired != SpecBindingOperatorActionChooseSection {
		t.Fatalf("operator_action_required = %q", result.OperatorActionRequired)
	}
}

func TestSpecBindingPreflightRequiresDraftSectionWhenNoHighConfidenceMatch(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{specBindingTestSection("TS.known.001", project.SpecSectionStateActive)},
	}
	result := BuildSpecBindingPreflight(
		specSet,
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle: "New governance relation outside known model",
			},
		},
	)

	if result.State != SpecBindingStateDraftNeeded {
		t.Fatalf("state = %q, want %q", result.State, SpecBindingStateDraftNeeded)
	}
	if result.MissingSectionProposal == nil {
		t.Fatalf("missing_section_proposal = nil, want draft proposal cue")
	}
}

func TestSpecBindingPreflightAllowsExplicitOutOfSpecWithDebt(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{specBindingTestSection("TS.known.001", project.SpecSectionStateActive)},
	}
	result := BuildSpecBindingPreflight(
		specSet,
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle:         "Emergency tactical bypass",
				Mode:                  SpecBindingDecisionModeTactical,
				BindingFallbackReason: "out-of-spec production incident workaround",
			},
		},
	)

	if result.State != SpecBindingStateOutOfSpec {
		t.Fatalf("state = %q, want %q", result.State, SpecBindingStateOutOfSpec)
	}
	if result.StatusDebt.Severity != SpecBindingDebtHigh {
		t.Fatalf("status_debt = %#v, want high debt", result.StatusDebt)
	}
}

func TestSpecBindingPreflightBlocksDeclaredConflict(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{specBindingTestSection("TS.known.001", project.SpecSectionStateActive)},
	}
	result := BuildSpecBindingPreflight(
		specSet,
		SpecBindingPreflightInput{
			DecisionDraft: SpecBindingDecisionDraft{
				SelectedTitle:    "Conflicting direction",
				DeclaredRelation: SpecBindingStateConflict,
				ConflictRefs:     []string{"TS.known.001"},
			},
		},
	)

	if result.State != SpecBindingStateConflict {
		t.Fatalf("state = %q, want %q", result.State, SpecBindingStateConflict)
	}
	if result.OperatorActionRequired != SpecBindingOperatorActionReopenProblem {
		t.Fatalf("operator_action_required = %q", result.OperatorActionRequired)
	}
}

func specBindingTestSection(id string, state project.SpecSectionState) project.SpecSection {
	return specBindingTestSectionWithTargets(id, state, nil)
}

func specBindingTestSectionWithTargets(
	id string,
	state project.SpecSectionState,
	targetRefs []string,
) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Kind:          "target.boundary",
		Title:         stringsForSpecBindingTestTitle(id),
		Status:        string(state),
		Terms:         []string{"decision-binding"},
		TargetRefs:    targetRefs,
		DocumentKind:  string(project.SpecDocumentKindTargetSystem),
		Spec:          "target-system",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
	}
}

func stringsForSpecBindingTestTitle(id string) string {
	switch id {
	case "TS.boundary.001":
		return "Spec use boundary"
	case "TS.other.001":
		return "Other boundary"
	default:
		return "Section " + id
	}
}

func containsSpecBindingTestString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
