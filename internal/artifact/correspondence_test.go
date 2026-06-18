package artifact

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildQualifiedCorrespondenceGraphKeepsPathNonProof(t *testing.T) {
	decision := correspondenceDecision()
	graph, err := BuildQualifiedCorrespondenceGraph(
		CorrespondenceGraphInput{DecisionRef: decision.Meta.ID},
		decision,
		[]AffectedFile{{Path: "internal/artifact/correspondence.go"}},
		[]EvidenceItem{correspondenceEvidence()},
		correspondenceNow(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if graph.PathStatus != CorrespondencePathNotProof {
		t.Fatalf("path_status = %q", graph.PathStatus)
	}
	if graph.AuthorityBoundary.Proof != CorrespondenceBoundaryNotProof {
		t.Fatalf("authority_boundary = %+v", graph.AuthorityBoundary)
	}
	if len(graph.ExpectedRealization) == 0 {
		t.Fatalf("expected realization is empty")
	}
	if len(graph.ObservedRealization) != 2 {
		t.Fatalf("observed realization = %#v", graph.ObservedRealization)
	}
	if !correspondenceHasEdge(graph, "CodeEntity--claimedToRealize-->Transformation") {
		t.Fatalf("missing code-to-transformation edge: %#v", graph.Edges)
	}
	if !correspondenceHasEdge(graph, "Observation--supportsViaEvidencePath-->Claim") {
		t.Fatalf("missing evidence-to-claim edge: %#v", graph.Edges)
	}
}

func TestBuildQualifiedCorrespondenceGraphReportsGaps(t *testing.T) {
	decision := correspondenceDecision()
	fields := decision.UnmarshalDecisionFields()
	fields.TransformationRecord = nil
	decision.StructuredData = mustMarshalCorrespondenceDecisionFields(fields)

	graph, err := BuildQualifiedCorrespondenceGraph(
		CorrespondenceGraphInput{DecisionRef: decision.Meta.ID},
		decision,
		nil,
		[]EvidenceItem{{ID: "evid-unbound", Verdict: "supports"}},
		correspondenceNow(),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		CorrespondenceGapMissingTransformation,
		CorrespondenceGapMissingAffectedFiles,
		CorrespondenceGapUnboundEvidence,
	} {
		if !correspondenceHasGap(graph, want) {
			t.Fatalf("missing gap %q in %#v", want, graph.Gaps)
		}
	}
}

func TestBuildQualifiedCorrespondenceGraphRejectsNonDecision(t *testing.T) {
	_, err := BuildQualifiedCorrespondenceGraph(
		CorrespondenceGraphInput{DecisionRef: "prob-1"},
		engineeringChangeCaseProblem(),
		nil,
		nil,
		correspondenceNow(),
	)
	if err == nil {
		t.Fatalf("expected non-decision error")
	}
}

func correspondenceDecision() *Artifact {
	fields := DecisionFields{
		SelectedTitle: "Add correspondence graph",
		Claims: []DecisionClaim{{
			ID:         "claim-1",
			Claim:      "Graph path remains non-proof",
			Observable: "path_status",
			Threshold:  "graph_path_not_proof",
			Status:     ClaimStatusUnverified,
		}},
		TransformationRecord: &TransformationRecord{
			SchemaVersion:     TransformationRecordSchemaVersion,
			TransformedEntity: "correspondence surface",
			InitialState:      "implicit scattered refs",
			PostState:         "qualified graph projection",
			Relation:          "projection_addition",
			Context:           "semantic spine",
		},
	}

	return &Artifact{
		Meta: Meta{
			ID:     "dec-1",
			Kind:   KindDecisionRecord,
			Status: StatusActive,
			Title:  "Add correspondence graph",
		},
		StructuredData: mustMarshalCorrespondenceDecisionFields(fields),
	}
}

func correspondenceEvidence() EvidenceItem {
	return EvidenceItem{
		ID:        "evid-1",
		Type:      "test",
		Verdict:   "supports",
		ClaimRefs: []string{"claim-1"},
	}
}

func correspondenceHasEdge(graph QualifiedCorrespondenceGraph, relation string) bool {
	for _, edge := range graph.Edges {
		if edge.RelationKind == relation && edge.PathStatus == CorrespondencePathNotProof {
			return true
		}
	}

	return false
}

func correspondenceHasGap(graph QualifiedCorrespondenceGraph, kind string) bool {
	for _, gap := range graph.Gaps {
		if gap.Kind == kind {
			return true
		}
	}

	return false
}

func correspondenceNow() time.Time {
	return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
}

func mustMarshalCorrespondenceDecisionFields(fields DecisionFields) string {
	data, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}

	return string(data)
}
