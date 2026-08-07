package codebase

import "testing"

func TestPartitionEdgeResolutionsAdmitsOnlyResolved(t *testing.T) {
	outcomes := []EdgeResolution{
		ResolvedEdge{Edge: CodeEdge{
			SrcID:      "caller",
			DstID:      "callee",
			Kind:       EdgeCall,
			FilePath:   "src/app.ts",
			Line:       10,
			Provenance: ProvenanceStatic,
		}},
		AmbiguousEdge{
			SourceID:     "caller",
			Kind:         EdgeCall,
			FilePath:     "src/app.ts",
			Line:         11,
			Reason:       ResolutionReasonMultipleCandidates,
			CandidateIDs: []string{"first", "second"},
		},
		UnresolvedEdge{
			SourceID: "caller",
			Kind:     EdgeCall,
			FilePath: "src/app.ts",
			Line:     12,
			Reason:   ResolutionReasonNoCandidate,
		},
	}

	edges, diagnostics := PartitionEdgeResolutions(outcomes)
	if len(edges) != 1 || edges[0].DstID != "callee" {
		t.Fatalf("admitted edges = %+v, want one resolved edge", edges)
	}
	if edges[0].Origin != EdgeOriginLegacyStatic || edges[0].Confidence != ConfidenceHigh {
		t.Fatalf("legacy edge metadata not normalized: %+v", edges[0])
	}
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %+v, want ambiguous+unresolved", diagnostics)
	}
	if diagnostics[0].Status != ResolutionAmbiguous || len(diagnostics[0].CandidateIDs) != 2 {
		t.Fatalf("ambiguous diagnostic = %+v", diagnostics[0])
	}
	if diagnostics[1].Status != ResolutionUnresolved || diagnostics[1].Reason != ResolutionReasonNoCandidate {
		t.Fatalf("unresolved diagnostic = %+v", diagnostics[1])
	}
}
