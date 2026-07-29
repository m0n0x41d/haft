package specflow

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestPhaseRegistryIncludesSoftwareSystemSpine(t *testing.T) {
	expected := []PhaseID{
		PhaseTargetEnvironmentDraft,
		PhaseTargetRoleDraft,
		PhaseTargetBoundaryDraft,
		PhaseSoftwareRoleDraft,
		PhaseSoftwareResponsibilityAllocationDraft,
		PhaseSoftwareFunctionalBehaviorDraft,
		PhaseSoftwareProceduralBehaviorDraft,
		PhaseSoftwareInterfacesDraft,
		PhaseSoftwareConstraintsDraft,
		PhaseSoftwareSelectedStructureDraft,
	}

	registry := PhaseRegistry()
	if len(registry) != len(expected) {
		t.Fatalf("registry length = %d, want %d", len(registry), len(expected))
	}
	for index, phase := range registry {
		if phase.ID != expected[index] {
			t.Fatalf("registry[%d] = %q, want %q", index, phase.ID, expected[index])
		}
	}
}

func TestSoftwarePhasesUseSoftwareDocumentAndReachTargetBoundary(t *testing.T) {
	for _, phase := range PhaseRegistry()[3:] {
		if phase.DocumentKind != project.SpecDocumentKindSoftwareSystem {
			t.Fatalf("phase %q document = %q", phase.ID, phase.DocumentKind)
		}
		if !phaseChainReaches(phase, PhaseTargetBoundaryDraft) {
			t.Fatalf("phase %q does not depend on target boundary", phase.ID)
		}
		for _, marker := range []string{"FRAME-", "CHR-", "VER-", "X-"} {
			if strings.Contains(phase.PromptForUser, marker) {
				t.Fatalf("phase %q leaks internal pattern %q", phase.ID, marker)
			}
		}
	}
}

func TestNextStepCompletesSoftwareSystemSpine(t *testing.T) {
	sections := []project.SpecSection{
		activeEnvironmentSection(),
		activeRoleSection(),
		activeBoundarySection(),
		activeSoftwareSection("software.role", "SS.role.001"),
		activeSoftwareSection("software.responsibility_allocation", "SS.allocation.001"),
		activeSoftwareSection("software.functional_behavior", "SS.functional.001"),
		activeSoftwareSection("software.procedural_behavior", "SS.procedural.001"),
		activeSoftwareSection("software.interfaces", "SS.interfaces.001"),
		activeSoftwareSection("software.constraints", "SS.constraints.001"),
		activeSoftwareSection("software.selected_structure", "SS.structure.001"),
	}

	intent := NextStep(DeriveState(project.ProjectSpecificationSet{Sections: sections}))
	if !intent.Terminal {
		t.Fatalf("intent = %+v, want terminal", intent)
	}
}

func TestNextStepCompletesRequiredSoftwareSpineWithoutOptionalSections(t *testing.T) {
	sections := []project.SpecSection{
		activeEnvironmentSection(),
		activeRoleSection(),
		activeBoundarySection(),
		activeSoftwareSection("software.role", "SS.role.001"),
		activeSoftwareSection("software.functional_behavior", "SS.functional.001"),
		activeSoftwareSection("software.interfaces", "SS.interfaces.001"),
		activeSoftwareSection("software.constraints", "SS.constraints.001"),
	}

	intent := NextStep(DeriveState(project.ProjectSpecificationSet{Sections: sections}))
	if !intent.Terminal || intent.Reason != "all required phases satisfied" {
		t.Fatalf("intent = %+v, want required-spine terminal", intent)
	}
}

func TestOptionalSoftwarePhasesStayDiscoverableWithoutBlockingOnboarding(t *testing.T) {
	optional := map[PhaseID]bool{
		PhaseSoftwareResponsibilityAllocationDraft: true,
		PhaseSoftwareProceduralBehaviorDraft:       true,
		PhaseSoftwareSelectedStructureDraft:        true,
	}

	for _, phase := range PhaseRegistry() {
		if optional[phase.ID] && phase.Required {
			t.Fatalf("optional phase %q marked required", phase.ID)
		}
	}
	for phaseID := range optional {
		if _, ok := FindPhase(phaseID); !ok {
			t.Fatalf("optional phase %q is not discoverable", phaseID)
		}
	}
}

func phaseChainReaches(phase Phase, target PhaseID) bool {
	for _, dependency := range phase.DependsOn {
		if dependency == target {
			return true
		}
		next, ok := FindPhase(dependency)
		if ok && phaseChainReaches(next, target) {
			return true
		}
	}
	return false
}

func activeSoftwareSection(kind string, id string) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          "software-system",
		DocumentKind:  "software-system",
		Kind:          kind,
		Title:         kind,
		Owner:         "human",
		Status:        string(project.SpecSectionStateActive),
		StatementType: "definition",
		ClaimLayer:    "object",
		ValidUntil:    "2026-12-31",
	}
}
