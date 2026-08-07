package specflow

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestSoftwareApplicablePhaseSetStartsTargetLifecycleWithoutEntityRelation(
	t *testing.T,
) {
	phaseSet := mustApplicablePhaseSet(t, softwareProfileScope(t, "software"))

	intent, err := NextStepForPhaseSet(
		DeriveState(project.ProjectSpecificationSet{}),
		phaseSet,
	)
	if err != nil {
		t.Fatalf("NextStepForPhaseSet: %v", err)
	}
	if intent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("phase = %q, want %q", intent.Phase, PhaseTargetEnvironmentDraft)
	}
	if intent.ApplicabilityCue != nil {
		t.Fatalf("target lifecycle retained relation cue: %#v", intent.ApplicabilityCue)
	}
	for _, phase := range phaseSet.Phases() {
		if phase.ID == PhaseSoftwareRoleDraft &&
			!containsPhaseID(phase.DependsOn, PhaseTargetBoundaryDraft) {
			t.Fatalf(
				"software role phase silently deleted target prerequisite: %#v",
				phase.DependsOn,
			)
		}
	}
}

func TestSoftwareApplicablePhaseSetReturnsToMissingTargetLifecycle(
	t *testing.T,
) {
	phaseSet := mustApplicablePhaseSet(t, softwareProfileScope(t, "software"))
	sections := []project.SpecSection{
		activeSoftwareSection("software.role", "SS.role.001"),
		activeSoftwareSection("software.functional_behavior", "SS.functional.001"),
		activeSoftwareSection("software.interfaces", "SS.interfaces.001"),
		activeSoftwareSection("software.constraints", "SS.constraints.001"),
	}
	state := DeriveState(project.ProjectSpecificationSet{Sections: sections})

	projection, err := ProjectLifecycleForPhaseSet(state, phaseSet)
	if err != nil {
		t.Fatalf("ProjectLifecycleForPhaseSet: %v", err)
	}
	if projection.State != LifecycleStateNeedsAction {
		t.Fatalf("state = %q, want needs_action", projection.State)
	}
	if projection.Action != LifecycleActionDraft {
		t.Fatalf("action = %q, want draft", projection.Action)
	}
	if projection.HumanGate != "" {
		t.Fatalf("neutral applicability cue became human gate %q", projection.HumanGate)
	}
	if projection.WorkflowIntent.ApplicabilityCue != nil ||
		projection.WorkflowIntent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("target lifecycle projection = %#v", projection.WorkflowIntent)
	}
}

func TestNonSoftwareApplicablePhaseSetHasNoSoftwarePhaseOrSWEDebt(
	t *testing.T,
) {
	phaseSet := mustApplicablePhaseSet(
		t,
		nonSoftwareProfileScope(t, "documents"),
	)
	for _, phase := range phaseSet.Phases() {
		if phase.DocumentKind == project.SpecDocumentKindSoftwareSystem {
			t.Fatalf("non-software phase set retained SWE phase %q", phase.ID)
		}
	}

	projection, err := ProjectLifecycleForPhaseSet(
		DeriveState(project.ProjectSpecificationSet{}),
		phaseSet,
	)
	if err != nil {
		t.Fatalf("ProjectLifecycleForPhaseSet: %v", err)
	}
	if projection.State != LifecycleStateNeedsAction {
		t.Fatalf("state = %q, want needs_action", projection.State)
	}
	if projection.DocumentKind == project.SpecDocumentKindSoftwareSystem {
		t.Fatalf("projection retained software pressure: %#v", projection)
	}
	if projection.DocumentKind != project.SpecDocumentKindTargetSystem {
		t.Fatalf("projection omitted target lifecycle: %#v", projection)
	}
}

func TestZeroApplicabilityCannotCreateAnEmptyReadyPhaseSet(t *testing.T) {
	_, err := DeriveApplicablePhaseSet(
		project.ProjectSpecificationSetApplicability{},
	)
	if err == nil {
		t.Fatal("zero applicability produced a phase set")
	}
}

func containsPhaseID(values []PhaseID, expected PhaseID) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func mustApplicablePhaseSet(
	t *testing.T,
	scope projectprofile.RealizationScope,
) ApplicablePhaseSet {
	t.Helper()
	scopeSet, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{scope},
	)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopeSet)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	applicability, err := project.DeriveProjectSpecificationSetApplicability(
		matrix,
		scope.ScopeID(),
	)
	if err != nil {
		t.Fatalf("DeriveProjectSpecificationSetApplicability: %v", err)
	}
	phaseSet, err := DeriveApplicablePhaseSet(applicability)
	if err != nil {
		t.Fatalf("DeriveApplicablePhaseSet: %v", err)
	}
	return phaseSet
}

func softwareProfileScope(
	t *testing.T,
	rawScopeID string,
) projectprofile.SoftwareRealization {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	return scope
}

func nonSoftwareProfileScope(
	t *testing.T,
	rawScopeID string,
) projectprofile.NonSoftwareRealization {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	kindRef, err := projectprofile.NewKindRef("U.Episteme")
	if err != nil {
		t.Fatalf("NewKindRef: %v", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		projectprofile.NewReferencedKindOrientation(kindRef),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	return scope
}
