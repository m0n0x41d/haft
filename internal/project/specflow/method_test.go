package specflow

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestNextStepReturnsFirstPhaseOnEmptyProject(t *testing.T) {
	state := DeriveState(project.ProjectSpecificationSet{})

	intent := NextStep(state)

	if intent.Terminal {
		t.Fatalf("intent.Terminal = true on empty project; want first phase")
	}
	if intent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, PhaseTargetEnvironmentDraft)
	}
	if intent.DocumentKind != project.SpecDocumentKindTargetSystem {
		t.Fatalf("intent.DocumentKind = %q, want target-system", intent.DocumentKind)
	}
	if intent.Audience != AudienceAgent {
		t.Fatalf("intent.Audience = %q, want %q (no failing section yet)", intent.Audience, AudienceAgent)
	}
	if !containsString(intent.Checks, "require_statement_type") {
		t.Fatalf("intent.Checks should list the SoTA checks; got %v", intent.Checks)
	}
}

func TestNextStepBlocksOnDependencyWhenEnvironmentNotSatisfied(t *testing.T) {
	// Section exists for environment, but as draft with missing fields -> not satisfied.
	state := DeriveState(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			{
				ID:     "tgt-env-1",
				Kind:   "target.environment",
				Status: string(project.SpecSectionStateDraft),
			},
		},
	})

	intent := NextStep(state)

	if intent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want %q (env still in progress)", intent.Phase, PhaseTargetEnvironmentDraft)
	}
}

func TestNextStepAdvancesToRoleWhenEnvironmentSatisfied(t *testing.T) {
	state := DeriveState(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			activeEnvironmentSection(),
		},
	})

	intent := NextStep(state)

	if intent.Phase != PhaseTargetRoleDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, PhaseTargetRoleDraft)
	}
}

func TestNextStepReachesBoundaryAfterRoleSatisfied(t *testing.T) {
	state := DeriveState(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			activeEnvironmentSection(),
			activeRoleSection(),
		},
	})

	intent := NextStep(state)

	if intent.Phase != PhaseTargetBoundaryDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, PhaseTargetBoundaryDraft)
	}
}

func TestNextStepIsTerminalWhenAllPhasesSatisfied(t *testing.T) {
	state := DeriveState(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			activeEnvironmentSection(),
			activeRoleSection(),
			activeBoundarySection(),
		},
	})

	intent := NextStep(state)

	if !intent.Terminal {
		t.Fatalf("intent.Terminal = false; want true after all phases satisfied. Phase=%q Reason=%q", intent.Phase, intent.Reason)
	}
	if intent.Reason == "" {
		t.Fatalf("terminal intent should carry a reason")
	}
}

func TestNextStepReportsHumanAudienceWhenSectionFails(t *testing.T) {
	// Active environment section but with missing statement_type -> blocking.
	bad := activeEnvironmentSection()
	bad.StatementType = "" // strip a required field

	state := DeriveState(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{bad},
	})

	intent := NextStep(state)

	if intent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, PhaseTargetEnvironmentDraft)
	}
	if intent.Audience != AudienceHuman {
		t.Fatalf("intent.Audience = %q, want %q for blocking findings", intent.Audience, AudienceHuman)
	}
	if len(intent.BlockingFindings) == 0 {
		t.Fatalf("expected blocking findings on failing section")
	}
}

func TestPhaseRegistryDoesNotEmbedFPFCitationsInPromptText(t *testing.T) {
	// FPF citations live in ContextForAgent (agent reasoning) only.
	// The user-facing prompt must not name FRAME-XX / CHR-XX / etc.
	for _, phase := range PhaseRegistry() {
		for _, marker := range []string{"FRAME-", "CHR-", "CMP-", "EXP-", "DEC-", "VER-", "X-"} {
			if strings.Contains(phase.PromptForUser, marker) {
				t.Fatalf("phase %q PromptForUser leaks FPF citation %q: %q", phase.ID, marker, phase.PromptForUser)
			}
		}
	}
}

func TestPhaseRegistryRequiresSoTAChecksOnTargetPhases(t *testing.T) {
	required := []string{
		"require_statement_type",
		"require_claim_layer",
		"require_valid_until",
	}

	for _, phase := range PhaseRegistry() {
		names := phaseCheckNames(phase)
		for _, expected := range required {
			if !containsString(names, expected) {
				t.Fatalf("phase %q is missing required SoTA check %q (has %v)", phase.ID, expected, names)
			}
		}
	}
}

func TestFindPhaseRoundTripsRegistry(t *testing.T) {
	for _, phase := range PhaseRegistry() {
		got, ok := FindPhase(phase.ID)
		if !ok {
			t.Fatalf("FindPhase(%q) = !ok", phase.ID)
		}
		if got.ID != phase.ID {
			t.Fatalf("FindPhase(%q).ID = %q", phase.ID, got.ID)
		}
	}

	if _, ok := FindPhase("does.not.exist"); ok {
		t.Fatalf("FindPhase('does.not.exist') = ok; want missing")
	}
}

func activeEnvironmentSection() project.SpecSection {
	return project.SpecSection{
		ID:            "tgt-env-1",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "Environment change",
		Owner:         "human",
		Status:        string(project.SpecSectionStateActive),
		StatementType: "definition",
		ClaimLayer:    "object",
		ValidUntil:    "2026-10-28",
	}
}

func activeRoleSection() project.SpecSection {
	return project.SpecSection{
		ID:            "tgt-role-1",
		Spec:          "target-system",
		Kind:          "target.role",
		Title:         "Target system role",
		Owner:         "human",
		Status:        string(project.SpecSectionStateActive),
		StatementType: "definition",
		ClaimLayer:    "object",
		ValidUntil:    "2026-10-28",
		DependsOn:     []string{"tgt-env-1"},
	}
}

func activeBoundarySection() project.SpecSection {
	return project.SpecSection{
		ID:            "tgt-boundary-1",
		Spec:          "target-system",
		Kind:          "target.boundary",
		Title:         "Boundary norms",
		Owner:         "human",
		Status:        string(project.SpecSectionStateActive),
		StatementType: "rule",
		ClaimLayer:    "object",
		ValidUntil:    "2026-10-28",
		DependsOn:     []string{"tgt-role-1"},
		TargetRefs:    []string{"law", "admissibility", "deontics", "evidence"},
	}
}
