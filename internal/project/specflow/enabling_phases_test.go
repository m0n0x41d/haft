package specflow

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestPhaseRegistryIncludesEnablingSpine(t *testing.T) {
	expected := []PhaseID{
		PhaseTargetEnvironmentDraft,
		PhaseTargetRoleDraft,
		PhaseTargetBoundaryDraft,
		PhaseEnablingArchitectureDraft,
		PhaseEnablingWorkMethodsDraft,
		PhaseEnablingEffectBoundariesDraft,
		PhaseEnablingAgentPolicyDraft,
		PhaseEnablingCommissionPolicyDraft,
		PhaseEnablingRuntimePolicyDraft,
		PhaseEnablingEvidencePolicyDraft,
	}

	got := make([]PhaseID, 0, len(PhaseRegistry()))
	for _, phase := range PhaseRegistry() {
		got = append(got, phase.ID)
	}

	if len(got) != len(expected) {
		t.Fatalf("registry length = %d, want %d (got %v)", len(got), len(expected), got)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("registry[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestEnablingPhasesDependOnTargetBoundary(t *testing.T) {
	enablingIDs := []PhaseID{
		PhaseEnablingArchitectureDraft,
		PhaseEnablingWorkMethodsDraft,
		PhaseEnablingEffectBoundariesDraft,
		PhaseEnablingAgentPolicyDraft,
		PhaseEnablingCommissionPolicyDraft,
		PhaseEnablingRuntimePolicyDraft,
		PhaseEnablingEvidencePolicyDraft,
	}

	for _, id := range enablingIDs {
		phase, ok := FindPhase(id)
		if !ok {
			t.Fatalf("FindPhase(%q) = !ok", id)
		}
		if phase.DocumentKind != project.SpecDocumentKindEnablingSystem {
			t.Fatalf("phase %q DocumentKind = %q, want %q", id, phase.DocumentKind, project.SpecDocumentKindEnablingSystem)
		}
		if !chainReachesTarget(phase, PhaseTargetBoundaryDraft) {
			t.Fatalf("phase %q must transitively depend on %q (DependsOn=%v)", id, PhaseTargetBoundaryDraft, phase.DependsOn)
		}
	}
}

func chainReachesTarget(phase Phase, target PhaseID) bool {
	for _, dep := range phase.DependsOn {
		if dep == target {
			return true
		}
		next, ok := FindPhase(dep)
		if !ok {
			continue
		}
		if chainReachesTarget(next, target) {
			return true
		}
	}
	return false
}

func TestEnablingPhasesRequireSoTAChecks(t *testing.T) {
	enablingIDs := []PhaseID{
		PhaseEnablingArchitectureDraft,
		PhaseEnablingWorkMethodsDraft,
		PhaseEnablingEffectBoundariesDraft,
		PhaseEnablingAgentPolicyDraft,
		PhaseEnablingCommissionPolicyDraft,
		PhaseEnablingRuntimePolicyDraft,
		PhaseEnablingEvidencePolicyDraft,
	}

	required := []string{"require_statement_type", "require_claim_layer", "require_valid_until"}

	for _, id := range enablingIDs {
		phase, _ := FindPhase(id)
		names := phaseCheckNames(phase)
		for _, expected := range required {
			if !containsString(names, expected) {
				t.Fatalf("phase %q is missing SoTA check %q (has %v)", id, expected, names)
			}
		}
	}
}

func TestEnablingEffectBoundariesRequiresFourPerspectives(t *testing.T) {
	phase, _ := FindPhase(PhaseEnablingEffectBoundariesDraft)
	names := phaseCheckNames(phase)
	if !containsString(names, "require_boundary_perspectives:min=4") {
		t.Fatalf("effect_boundaries phase must compose CHR-10 boundary perspectives check; got %v", names)
	}
}

func TestEnablingEvidencePolicyRequiresGuardLocation(t *testing.T) {
	phase, _ := FindPhase(PhaseEnablingEvidencePolicyDraft)
	names := phaseCheckNames(phase)
	if !containsString(names, "require_guard_location") {
		t.Fatalf("evidence_policy phase must compose guard_location check; got %v", names)
	}
}

func TestEnablingPhasePromptsDoNotEmbedFPFCitations(t *testing.T) {
	enablingIDs := []PhaseID{
		PhaseEnablingArchitectureDraft,
		PhaseEnablingWorkMethodsDraft,
		PhaseEnablingEffectBoundariesDraft,
		PhaseEnablingAgentPolicyDraft,
		PhaseEnablingCommissionPolicyDraft,
		PhaseEnablingRuntimePolicyDraft,
		PhaseEnablingEvidencePolicyDraft,
	}

	for _, id := range enablingIDs {
		phase, _ := FindPhase(id)
		for _, marker := range []string{"FRAME-", "CHR-", "CMP-", "EXP-", "DEC-", "VER-", "X-"} {
			if strings.Contains(phase.PromptForUser, marker) {
				t.Fatalf("phase %q PromptForUser leaks FPF citation %q: %q", id, marker, phase.PromptForUser)
			}
		}
	}
}

func TestNextStepReachesEnablingSpineAfterTargetSatisfied(t *testing.T) {
	state := DeriveState(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			activeEnvironmentSection(),
			activeRoleSection(),
			activeBoundarySection(),
		},
	})

	intent := NextStep(state)

	if intent.Phase != PhaseEnablingArchitectureDraft {
		t.Fatalf("intent.Phase = %q, want %q (target satisfied -> first enabling phase)", intent.Phase, PhaseEnablingArchitectureDraft)
	}
	if intent.DocumentKind != project.SpecDocumentKindEnablingSystem {
		t.Fatalf("DocumentKind = %q, want enabling-system", intent.DocumentKind)
	}
}

func TestNextStepReachesTerminalAfterFullSpine(t *testing.T) {
	sections := []project.SpecSection{
		activeEnvironmentSection(),
		activeRoleSection(),
		activeBoundarySection(),
		activeEnablingSection("enabling.architecture", "ena-arch-1"),
		activeEnablingSection("enabling.work_methods", "ena-work-1"),
		activeEnablingEffectBoundarySection(),
		activeEnablingSection("enabling.agent_policy", "ena-agent-1"),
		activeEnablingSection("enabling.commission_policy", "ena-commission-1"),
		activeEnablingSection("enabling.runtime_policy", "ena-runtime-1"),
		activeEnablingEvidencePolicySection(),
	}

	intent := NextStep(DeriveState(project.ProjectSpecificationSet{Sections: sections}))

	if !intent.Terminal {
		t.Fatalf("intent.Terminal = false; want true after full spine. Phase=%q Reason=%q", intent.Phase, intent.Reason)
	}
}

func activeEnablingSection(kind, id string) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          "enabling-system",
		Kind:          kind,
		Title:         kind + " section",
		Owner:         "human",
		Status:        string(project.SpecSectionStateActive),
		StatementType: "duty",
		ClaimLayer:    "work",
		ValidUntil:    "2026-12-31",
		DependsOn:     []string{"tgt-boundary-1"},
	}
}

func activeEnablingEffectBoundarySection() project.SpecSection {
	section := activeEnablingSection("enabling.effect_boundaries", "ena-effect-1")
	section.TargetRefs = []string{"law", "admissibility", "deontics", "evidence"}
	return section
}

func activeEnablingEvidencePolicySection() project.SpecSection {
	section := activeEnablingSection("enabling.evidence_policy", "ena-evidence-1")
	section.EvidenceRequired = []project.SpecEvidenceRequirement{
		{Kind: "E2E", Description: "end-to-end harness pass"},
	}
	return section
}
