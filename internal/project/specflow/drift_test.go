package specflow

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestSectionBaselineFindings_NoStoreNoFindings(t *testing.T) {
	set := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{activeEnvironmentSection()},
	}

	findings := SectionBaselineFindings(set, nil, "proj-1")
	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 with nil store", len(findings))
	}
}

func TestSectionBaselineFindings_DraftSectionsAreSkipped(t *testing.T) {
	store := NewMemoryBaselineStore()
	draft := activeEnvironmentSection()
	draft.Status = string(project.SpecSectionStateDraft)

	set := project.ProjectSpecificationSet{Sections: []project.SpecSection{draft}}

	findings := SectionBaselineFindings(set, store, "proj-1")
	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 for draft section", len(findings))
	}
}

func TestSectionBaselineFindings_ActiveWithoutBaselineFlagsNeedsBaseline(t *testing.T) {
	store := NewMemoryBaselineStore()
	set := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{activeEnvironmentSection()},
	}

	findings := SectionBaselineFindings(set, store, "proj-1")
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeSpecSectionNeedsBaseline {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeSpecSectionNeedsBaseline)
	}
}

func TestSectionBaselineFindings_MatchingBaselineProducesNoFinding(t *testing.T) {
	store := NewMemoryBaselineStore()
	section := activeEnvironmentSection()
	store.Put(SectionBaseline{
		ProjectID: "proj-1",
		SectionID: section.ID,
		Hash:      HashSection(section),
	})

	set := project.ProjectSpecificationSet{Sections: []project.SpecSection{section}}
	findings := SectionBaselineFindings(set, store, "proj-1")
	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 with matching baseline", len(findings))
	}
}

func TestSectionBaselineFindings_DriftedHashFlagsDrifted(t *testing.T) {
	store := NewMemoryBaselineStore()
	section := activeEnvironmentSection()
	store.Put(SectionBaseline{
		ProjectID: "proj-1",
		SectionID: section.ID,
		Hash:      "stale-hash",
	})

	set := project.ProjectSpecificationSet{Sections: []project.SpecSection{section}}
	findings := SectionBaselineFindings(set, store, "proj-1")
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeSpecSectionDrifted {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeSpecSectionDrifted)
	}
	if !strings.Contains(findings[0].NextAction, "rebaseline") {
		t.Fatalf("next_action should mention rebaseline; got %q", findings[0].NextAction)
	}
}

func TestNextStepWithBaselinesBlocksOnMissingBaseline(t *testing.T) {
	store := NewMemoryBaselineStore()
	set := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{activeEnvironmentSection()},
	}
	state := DeriveStateWithBaselines(set, store, "proj-1")

	intent := NextStep(state)

	if intent.Terminal {
		t.Fatalf("intent.Terminal = true; want first phase blocked on missing baseline")
	}
	if intent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, PhaseTargetEnvironmentDraft)
	}
	if intent.Audience != AudienceHuman {
		t.Fatalf("intent.Audience = %q, want %q (blocking findings)", intent.Audience, AudienceHuman)
	}

	codes := make(map[string]int)
	for _, f := range intent.BlockingFindings {
		codes[f.Code]++
	}
	if codes[codeSpecSectionNeedsBaseline] == 0 {
		t.Fatalf("expected %s in blocking findings; got %v", codeSpecSectionNeedsBaseline, codes)
	}
}

func TestNextStepWithBaselinesAdvancesWhenBaselineMatches(t *testing.T) {
	store := NewMemoryBaselineStore()
	section := activeEnvironmentSection()
	store.Put(SectionBaseline{
		ProjectID: "proj-1",
		SectionID: section.ID,
		Hash:      HashSection(section),
	})

	set := project.ProjectSpecificationSet{Sections: []project.SpecSection{section}}
	state := DeriveStateWithBaselines(set, store, "proj-1")

	intent := NextStep(state)

	if intent.Phase != PhaseTargetRoleDraft {
		t.Fatalf("intent.Phase = %q, want %q after env baseline matches", intent.Phase, PhaseTargetRoleDraft)
	}
}

func TestNextStepWithBaselinesBlocksOnDriftedBaseline(t *testing.T) {
	store := NewMemoryBaselineStore()
	section := activeEnvironmentSection()
	store.Put(SectionBaseline{
		ProjectID: "proj-1",
		SectionID: section.ID,
		Hash:      "old-hash-from-earlier-approval",
	})

	set := project.ProjectSpecificationSet{Sections: []project.SpecSection{section}}
	state := DeriveStateWithBaselines(set, store, "proj-1")

	intent := NextStep(state)

	if intent.Phase != PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want env phase blocked on drift", intent.Phase)
	}
	codes := make(map[string]int)
	for _, f := range intent.BlockingFindings {
		codes[f.Code]++
	}
	if codes[codeSpecSectionDrifted] == 0 {
		t.Fatalf("expected %s in blocking findings; got %v", codeSpecSectionDrifted, codes)
	}
}
