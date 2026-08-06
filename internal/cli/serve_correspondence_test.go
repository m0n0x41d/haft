package cli

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQuery_CorrespondenceGraphKeepsPathNonProof(t *testing.T) {
	t.Parallel()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Expose qualified correspondence",
		ProblemStatement: "Expected-vs-observed graph paths need a non-proof projection boundary.",
		WhySelected:      "Need expected-vs-observed graph paths without proof authority.",
		SelectionPolicy:  "Prefer read-only projection over canonical relation mutation.",
		CounterArgument:  "A stored typed graph would support richer repair routing later.",
		WeakestLink:      "Declared file refs can be stale or incomplete.",
		TransformationRecord: &artifact.TransformationRecord{
			SchemaVersion:     artifact.TransformationRecordSchemaVersion,
			TransformedEntity: "correspondence surface",
			InitialState:      "implicit refs",
			PostState:         "qualified projection",
			Relation:          "projection_addition",
			Context:           "semantic spine",
		},
		Predictions: []artifact.PredictionInput{{
			Claim:      "Graph path remains non-proof",
			Observable: "path status",
			Threshold:  "graph_path_not_proof",
		}},
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Stored canonical graph",
			Reason:  "Rejected until repair routing needs durable graph identity.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Correspondence graph implies proof."},
		},
		ValidUntil: time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SetAffectedFiles(ctx, decision.Meta.ID, []artifact.AffectedFile{{
		Path: "internal/artifact/correspondence.go",
	}}); err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Get(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	claims := reloaded.UnmarshalDecisionFields().Claims
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %+v", claims)
	}

	_, err = artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef:     decision.Meta.ID,
		Content:         "Projection test validates non-proof path status.",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  7,
		ClaimRefs:       []string{claims[0].ID},
		ValidUntil:      time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, haftDir, map[string]any{
		"action":       "correspondence_graph",
		"artifact_ref": decision.Meta.ID,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery correspondence_graph returned error: %v", err)
	}

	var graph artifact.QualifiedCorrespondenceGraph
	if err := json.Unmarshal([]byte(result), &graph); err != nil {
		t.Fatalf("decode correspondence graph: %v\n%s", err, result)
	}

	if graph.PathStatus != artifact.CorrespondencePathNotProof {
		t.Fatalf("path_status = %q", graph.PathStatus)
	}
	if graph.AuthorityBoundary.Proof != artifact.CorrespondenceBoundaryNotProof {
		t.Fatalf("authority boundary = %+v", graph.AuthorityBoundary)
	}
	if len(graph.ExpectedRealization) == 0 || len(graph.ObservedRealization) == 0 {
		t.Fatalf("expected=%#v observed=%#v", graph.ExpectedRealization, graph.ObservedRealization)
	}
	if !serveCorrespondenceHasEdge(graph, "CodeEntity--claimedToRealize-->Transformation") {
		t.Fatalf("missing code correspondence edge: %#v", graph.Edges)
	}
	if !serveCorrespondenceHasEdge(graph, "Observation--supportsViaEvidencePath-->Claim") {
		t.Fatalf("missing evidence correspondence edge: %#v", graph.Edges)
	}
}

func serveCorrespondenceHasEdge(
	graph artifact.QualifiedCorrespondenceGraph,
	relation string,
) bool {
	for _, edge := range graph.Edges {
		if edge.RelationKind == relation && edge.PathStatus == artifact.CorrespondencePathNotProof {
			return true
		}
	}

	return false
}
