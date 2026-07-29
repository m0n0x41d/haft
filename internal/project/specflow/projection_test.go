package specflow

import (
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestProjectLifecycleEmptyProjectDraftsFirstSection(t *testing.T) {
	state := DeriveState(project.ProjectSpecificationSet{})

	projection := ProjectLifecycle(state)

	if projection.State != LifecycleStateNeedsAction {
		t.Fatalf("State = %q, want %q", projection.State, LifecycleStateNeedsAction)
	}
	if projection.Action != LifecycleActionDraft {
		t.Fatalf("Action = %q, want %q", projection.Action, LifecycleActionDraft)
	}
	if projection.Carrier != ".haft/specs/target-system.md" {
		t.Fatalf("Carrier = %q", projection.Carrier)
	}
	if !slices.Contains(projection.AllowedCommands, "haft spec check") {
		t.Fatalf("AllowedCommands = %#v, want haft spec check", projection.AllowedCommands)
	}
	if projection.WorkflowIntent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("WorkflowIntent.Phase = %q", projection.WorkflowIntent.Phase)
	}
}

func TestProjectLifecycleBlocksOnLegacySpecMigration(t *testing.T) {
	finding := project.SpecCheckFinding{
		Level:   "error",
		Code:    project.SpecMigrationRequiredFindingCode,
		Path:    ".haft/specs/enabling-system.md",
		Message: "legacy spec requires migration",
	}
	state := DeriveState(project.ProjectSpecificationSet{Findings: []project.SpecCheckFinding{finding}})

	projection := ProjectLifecycle(state)

	if projection.Action != LifecycleActionClarify || projection.State != LifecycleStateNeedsAction {
		t.Fatalf("projection = %#v", projection)
	}
	if projection.Object != "ProjectSpecificationSet" || projection.Carrier != finding.Path {
		t.Fatalf("projection = %#v", projection)
	}
	if !slices.Contains(projection.AllowedCommands, "haft spec migrate") {
		t.Fatalf("AllowedCommands = %#v", projection.AllowedCommands)
	}
}

func TestProjectLifecycleActiveSectionWithoutBaselineRequiresApprove(t *testing.T) {
	store := NewMemoryBaselineStore()
	state := DeriveStateWithBaselines(
		project.ProjectSpecificationSet{Sections: []project.SpecSection{activeEnvironmentSection()}},
		store,
		"proj-1",
	)

	projection := ProjectLifecycle(state)

	if projection.State != LifecycleStateNeedsHumanGate {
		t.Fatalf("State = %q, want %q", projection.State, LifecycleStateNeedsHumanGate)
	}
	if projection.Action != LifecycleActionApprove {
		t.Fatalf("Action = %q, want %q", projection.Action, LifecycleActionApprove)
	}
	if projection.Object != "SpecSectionBaseline" {
		t.Fatalf("Object = %q, want SpecSectionBaseline", projection.Object)
	}
	if projection.BaselineKind != BaselineKindSpecSectionApproval {
		t.Fatalf("BaselineKind = %q, want %q", projection.BaselineKind, BaselineKindSpecSectionApproval)
	}
	if projection.BaselineProfile == nil {
		t.Fatal("BaselineProfile missing")
	}
	if projection.BaselineProfile.Object != "SpecSectionApprovalBaseline" {
		t.Fatalf("BaselineProfile.Object = %q", projection.BaselineProfile.Object)
	}
	if !slices.Contains(projection.AllowedCommands, "haft spec approve tgt-env-1") {
		t.Fatalf("AllowedCommands = %#v, want approve command", projection.AllowedCommands)
	}
}

func TestProjectLifecycleMixedStructuralAndBaselineFindingsRequiresClarify(t *testing.T) {
	store := NewMemoryBaselineStore()
	section := activeEnvironmentSection()
	section.Title = ""
	state := DeriveStateWithBaselines(
		project.ProjectSpecificationSet{Sections: []project.SpecSection{section}},
		store,
		"proj-1",
	)

	projection := ProjectLifecycle(state)

	if projection.State != LifecycleStateNeedsAction {
		t.Fatalf("State = %q, want %q", projection.State, LifecycleStateNeedsAction)
	}
	if projection.Action != LifecycleActionClarify {
		t.Fatalf("Action = %q, want %q", projection.Action, LifecycleActionClarify)
	}
	if slices.Contains(projection.AllowedCommands, "haft spec approve tgt-env-1") {
		t.Fatalf("AllowedCommands = %#v, approve must stay blocked while structural findings remain", projection.AllowedCommands)
	}
}

func TestProjectLifecycleDriftedSectionRequiresTriage(t *testing.T) {
	store := NewMemoryBaselineStore()
	section := activeEnvironmentSection()
	store.Put(SectionBaseline{
		ProjectID: "proj-1",
		SectionID: section.ID,
		Hash:      "old-hash",
	})

	state := DeriveStateWithBaselines(
		project.ProjectSpecificationSet{Sections: []project.SpecSection{section}},
		store,
		"proj-1",
	)

	projection := ProjectLifecycle(state)

	if projection.State != LifecycleStateNeedsTriage {
		t.Fatalf("State = %q, want %q", projection.State, LifecycleStateNeedsTriage)
	}
	if projection.Action != LifecycleActionTriage {
		t.Fatalf("Action = %q, want %q", projection.Action, LifecycleActionTriage)
	}
	if projection.BaselineKind != BaselineKindSpecSectionApproval {
		t.Fatalf("BaselineKind = %q, want %q", projection.BaselineKind, BaselineKindSpecSectionApproval)
	}
	for _, want := range []string{
		"haft spec rebaseline tgt-env-1 --reason <reason>",
		"haft spec reopen tgt-env-1 --reason <reason>",
	} {
		if !slices.Contains(projection.AllowedCommands, want) {
			t.Fatalf("AllowedCommands = %#v, want %q", projection.AllowedCommands, want)
		}
	}
}

func TestProjectLifecycleTerminalIsReady(t *testing.T) {
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
	state := DeriveState(project.ProjectSpecificationSet{Sections: sections})

	projection := ProjectLifecycle(state)

	if projection.State != LifecycleStateReady {
		t.Fatalf("State = %q, want %q", projection.State, LifecycleStateReady)
	}
	if projection.Action != LifecycleActionNone {
		t.Fatalf("Action = %q, want %q", projection.Action, LifecycleActionNone)
	}
}
