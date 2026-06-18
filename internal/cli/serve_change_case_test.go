package cli

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQuery_ChangeCaseBuildsDerivedAggregate(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:                "Semantic case context is scattered",
		ProblemProfile:       artifact.ProblemProfileDeep,
		SourceKind:           artifact.ProblemSourceObserved,
		Signal:               "Problem, transformation, choice, and evidence are inspectable only as separate records.",
		WhyNow:               "The semantic-spine plan needs a derived case view.",
		Scope:                "One DecisionRecord and its directly referenced records.",
		AcceptanceProbe:      "A read-only projection lists refs without creating authority.",
		FreshnessDisposition: "Review after the current slice train.",
		Acceptance:           "Case projection includes problem, transformation, choice, evidence, and authority boundaries.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		PortfolioRef:    "sol-test",
		SelectedTitle:   "Derived read-only case",
		WhySelected:     "The case should improve review without creating a new root artifact.",
		SelectionPolicy: "Prefer derived projection over storage mutation.",
		CounterArgument: "A durable case identity would be better for long handoffs.",
		WeakestLink:     "Derived views can be incomplete when refs are missing.",
		Invariants:      []string{"Case projection is not approval, proof, or work occurrence."},
		PostConditions:  []string{"Change case can be inspected explicitly."},
		AffectedFiles:   []string{"internal/artifact/change_case.go"},
		ValidUntil:      time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339),
		TransformationRecord: &artifact.TransformationRecord{
			SchemaVersion:     artifact.TransformationRecordSchemaVersion,
			TransformedEntity: "governance review surface",
			InitialState:      "separate records only",
			PostState:         "derived case projection",
			Relation:          "aggregate_projection",
			Context:           "semantic spine",
		},
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Stored case artifact",
			Reason:  "Rejected until handoff/dispute pressure proves durable identity is needed.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Case projection implies approval or work occurrence."},
			Steps:    []string{"Remove the explicit change_case surface."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef:     decision.Meta.ID,
		Content:         "Projection test covers the aggregate contract.",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  7,
		ValidUntil:      time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, haftDir, map[string]any{
		"action":        "change_case",
		"artifact_ref":  decision.Meta.ID,
		"attempted_use": "implementation review",
		"method_ref":    "mpull-test-change-case",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery change_case returned error: %v", err)
	}

	var record artifact.EngineeringChangeCase
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode change_case record: %v\n%s", err, result)
	}

	if record.DecisionSpeechActRef != decision.Meta.ID {
		t.Fatalf("decision ref = %q, want %q", record.DecisionSpeechActRef, decision.Meta.ID)
	}
	if len(record.ProblemCardRefs) != 1 || record.ProblemCardRefs[0] != problem.Meta.ID {
		t.Fatalf("problem refs = %#v", record.ProblemCardRefs)
	}
	if len(record.Transformations) != 1 {
		t.Fatalf("transformations = %#v", record.Transformations)
	}
	if record.ChoiceResult == nil || record.ChoiceResult.VariantRef != "Derived read-only case" {
		t.Fatalf("choice_result = %#v", record.ChoiceResult)
	}
	if len(record.EvidenceItemRefs) != 1 || len(record.EvidencePathRefs) != 1 {
		t.Fatalf("evidence refs = %#v paths = %#v", record.EvidenceItemRefs, record.EvidencePathRefs)
	}
	if record.AuthorityBoundary.Proof != artifact.EngineeringChangeCaseBoundaryNotProof {
		t.Fatalf("authority boundary = %+v", record.AuthorityBoundary)
	}
	if record.AuthorityBoundary.GateDecision != artifact.EngineeringChangeCaseBoundaryNotGateDecision {
		t.Fatalf("authority boundary = %+v", record.AuthorityBoundary)
	}
	if record.AuthorityBoundary.WorkOccurrence != artifact.EngineeringChangeCaseBoundaryNotWorkOccurrence {
		t.Fatalf("authority boundary = %+v", record.AuthorityBoundary)
	}
}
