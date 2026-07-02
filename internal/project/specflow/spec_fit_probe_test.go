package specflow

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestBuildSpecFitProbeRelatesExistingSection(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{{
			ID:         "TS.fit.001",
			Status:     string(project.SpecSectionStateActive),
			Title:      "Checkout fit",
			TargetRefs: []string{"symbol:internal/checkout.go::Run"},
		}},
	}

	result := BuildSpecFitProbe(specSet, SpecFitProbeInput{
		ProblemSignal: "Checkout flow needs a decision.",
		TargetRefs:    []string{"symbol:internal/checkout.go::Run"},
	})

	if result.RecordKind != SpecFitProbeRecordKind {
		t.Fatalf("record_kind = %q", result.RecordKind)
	}
	if result.State != SpecFitStateRelatesExisting {
		t.Fatalf("state = %q, want relates_existing", result.State)
	}
	if len(result.CandidateSectionRefs) != 1 || result.CandidateSectionRefs[0] != "TS.fit.001" {
		t.Fatalf("candidate_section_refs = %#v", result.CandidateSectionRefs)
	}
	if result.NextExpectedAction != SpecFitNextOrdinaryExplore {
		t.Fatalf("next_expected_action = %q", result.NextExpectedAction)
	}
}

func TestBuildSpecFitProbeAggregatesVariantSpecFit(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{{
			ID:     "TS.fit.002",
			Status: string(project.SpecSectionStateActive),
			Title:  "Existing boundary",
			Terms:  []string{"boundary"},
		}},
	}

	result := BuildSpecFitProbe(specSet, SpecFitProbeInput{
		ProblemSignal: "Compare variants before deciding.",
		Variants: []SpecFitVariantInput{
			{
				ID:          "V1",
				Title:       "Use existing boundary",
				Description: "Keep the boundary term.",
				SectionRefs: []string{"TS.fit.002"},
			},
			{
				ID:               "V2",
				Title:            "Contradict boundary",
				DeclaredRelation: "conflict",
				ConflictRefs:     []string{"TS.fit.002"},
			},
		},
	})

	if result.State != SpecFitStateConflict {
		t.Fatalf("state = %q, want conflict", result.State)
	}
	if result.NextExpectedAction != SpecFitNextExploreSpecDelta {
		t.Fatalf("next_expected_action = %q", result.NextExpectedAction)
	}
	if len(result.VariantSpecFit) != 2 {
		t.Fatalf("variant_spec_fit = %#v", result.VariantSpecFit)
	}
	if result.VariantSpecFit[0].State != SpecFitStateRelatesExisting {
		t.Fatalf("V1 state = %q", result.VariantSpecFit[0].State)
	}
	if result.VariantSpecFit[1].State != SpecFitStateConflict {
		t.Fatalf("V2 state = %q", result.VariantSpecFit[1].State)
	}
}

func TestBuildSpecFitProbeReportsSpecGap(t *testing.T) {
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{{
			ID:     "TS.fit.003",
			Status: string(project.SpecSectionStateActive),
			Title:  "Existing unrelated section",
		}},
	}

	result := BuildSpecFitProbe(specSet, SpecFitProbeInput{
		ProblemSignal: "No existing section names this behavior.",
	})

	if result.State != SpecFitStateSpecGap {
		t.Fatalf("state = %q, want spec_gap", result.State)
	}
	if result.NextExpectedAction != SpecFitNextDraftSection {
		t.Fatalf("next_expected_action = %q", result.NextExpectedAction)
	}
}
