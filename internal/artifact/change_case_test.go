package artifact

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildEngineeringChangeCaseDerivesAggregateWithoutAuthority(t *testing.T) {
	decision := engineeringChangeCaseDecision()
	problem := engineeringChangeCaseProblem()
	evidence := []EvidenceItem{engineeringChangeCaseEvidence()}

	record, err := BuildEngineeringChangeCase(
		EngineeringChangeCaseInput{
			DecisionRef:  decision.Meta.ID,
			AttemptedUse: "implementation review",
			MethodRef:    "mpull-1",
		},
		decision,
		[]*Artifact{problem},
		evidence,
		engineeringChangeCaseNow(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if record.CaseRef != "change-case:dec-1" {
		t.Fatalf("case_ref = %q", record.CaseRef)
	}
	if len(record.ProblemCardRefs) != 1 || record.ProblemCardRefs[0] != "prob-1" {
		t.Fatalf("problem refs = %#v", record.ProblemCardRefs)
	}
	if len(record.Problems) != 1 || record.Problems[0].Readiness != ProblemReadinessReady {
		t.Fatalf("problems = %#v", record.Problems)
	}
	if len(record.TransformationRefs) != 1 || record.TransformationRefs[0] != "dec-1#transformation_record" {
		t.Fatalf("transformation refs = %#v", record.TransformationRefs)
	}
	if record.ChoiceResultRef != "dec-1#choice_result" {
		t.Fatalf("choice_result_ref = %q", record.ChoiceResultRef)
	}
	if record.CandidateSetRef != "sol-1" {
		t.Fatalf("candidate_set_ref = %q", record.CandidateSetRef)
	}
	if len(record.EvidencePathRefs) != 1 || record.EvidencePathRefs[0] != "dec-1#evidence_path:evid-1" {
		t.Fatalf("evidence_path_refs = %#v", record.EvidencePathRefs)
	}
	if record.EvidencePaths[0].RelianceDisposition.Disposition != EvidenceRelianceBounded {
		t.Fatalf("evidence_path = %+v, want bounded", record.EvidencePaths[0].RelianceDisposition)
	}
	if record.AuthorityBoundary.GateDecision != EngineeringChangeCaseBoundaryNotGateDecision {
		t.Fatalf("authority boundary = %+v", record.AuthorityBoundary)
	}
	if record.AuthorityBoundary.WorkOccurrence != EngineeringChangeCaseBoundaryNotWorkOccurrence {
		t.Fatalf("authority boundary = %+v", record.AuthorityBoundary)
	}
	if record.AuthorityBoundary.ClaimTruth != EngineeringChangeCaseBoundaryNotClaimTruth {
		t.Fatalf("authority boundary = %+v", record.AuthorityBoundary)
	}
	if record.AuthorityBoundary.Publication != EngineeringChangeCaseBoundaryNotPublication {
		t.Fatalf("authority boundary = %+v", record.AuthorityBoundary)
	}
}

func TestBuildEngineeringChangeCaseRequiresAttemptedUseForEvidencePaths(t *testing.T) {
	decision := engineeringChangeCaseDecision()

	record, err := BuildEngineeringChangeCase(
		EngineeringChangeCaseInput{DecisionRef: decision.Meta.ID},
		decision,
		nil,
		[]EvidenceItem{engineeringChangeCaseEvidence()},
		engineeringChangeCaseNow(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(record.EvidencePathRefs) != 0 {
		t.Fatalf("evidence_path_refs = %#v, want none without attempted use", record.EvidencePathRefs)
	}
	if record.NextAdmissibleMove != EngineeringChangeCaseNextDeclareUse {
		t.Fatalf("next_admissible_move = %q", record.NextAdmissibleMove)
	}
}

func TestBuildEngineeringChangeCaseRejectsNonDecision(t *testing.T) {
	_, err := BuildEngineeringChangeCase(
		EngineeringChangeCaseInput{DecisionRef: "prob-1"},
		engineeringChangeCaseProblem(),
		nil,
		nil,
		engineeringChangeCaseNow(),
	)
	if err == nil {
		t.Fatalf("expected non-decision error")
	}
}

func engineeringChangeCaseDecision() *Artifact {
	fields := DecisionFields{
		ProblemRefs: []string{"prob-1"},
		ChoiceResult: &ChoiceResult{
			SubjectRef:      "operator",
			OptionSet:       []string{"slice train", "big bang"},
			ComparisonBasis: []string{"selected slice train: lower migration risk"},
			ChoiceRule:      "minimize migration risk",
			NextMove:        ChoiceNextMoveChooseNow,
			VariantRef:      "slice train",
			ProblemRefs:     []string{"prob-1"},
			PortfolioRef:    "sol-1",
			Reason:          "lower migration risk",
		},
		TransformationRecord: &TransformationRecord{
			SchemaVersion:     TransformationRecordSchemaVersion,
			TransformedEntity: "spec review surface",
			InitialState:      "advisory prose",
			PostState:         "typed read-only packet",
			Relation:          "semantic_profile_upgrade",
			Context:           "semantic spine",
		},
		SelectedTitle: "slice train",
	}

	return &Artifact{
		Meta: Meta{
			ID:     "dec-1",
			Kind:   KindDecisionRecord,
			Status: StatusActive,
			Title:  "Use slice train",
		},
		StructuredData: mustMarshalDecisionFields(fields),
	}
}

func engineeringChangeCaseProblem() *Artifact {
	fields := ProblemFields{
		Profile: &ProblemCardProfile{
			Level:      ProblemProfileDeep,
			Readiness:  ProblemReadinessReady,
			SourceKind: ProblemSourceObserved,
		},
	}

	return &Artifact{
		Meta: Meta{
			ID:     "prob-1",
			Kind:   KindProblemCard,
			Status: StatusActive,
			Title:  "Semantic drift is ambiguous",
		},
		StructuredData: mustMarshalProblemFields(fields),
	}
}

func engineeringChangeCaseEvidence() EvidenceItem {
	return EvidenceItem{
		ID:              "evid-1",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ClaimRefs:       []string{"claim-1"},
		ValidUntil:      "2099-01-01",
	}
}

func engineeringChangeCaseNow() time.Time {
	return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
}

func mustMarshalDecisionFields(fields DecisionFields) string {
	data, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func mustMarshalProblemFields(fields ProblemFields) string {
	data, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}

	return string(data)
}
