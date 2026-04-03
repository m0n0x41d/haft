package agent

import (
	"reflect"
	"testing"
)

func TestDetectObservationFromTool_AssignsFormalityAndScope(t *testing.T) {
	item := DetectObservationFromTool("bash", "go test ./internal/agent ./internal/reff", "", false)
	if item == nil {
		t.Fatal("expected observation item")
	}

	if item.Formality != FormalityStructuredInformal {
		t.Fatalf("formality = %d, want %d", item.Formality, FormalityStructuredInformal)
	}

	wantScope := []string{"internal/agent", "internal/reff"}
	if !reflect.DeepEqual(item.ClaimScope, wantScope) {
		t.Fatalf("claim_scope = %v, want %v", item.ClaimScope, wantScope)
	}
}

func TestDetectObservationFromTool_ReadScope(t *testing.T) {
	item := DetectObservationFromTool("read", "internal/agent/cycle.go", "", false)
	if item == nil {
		t.Fatal("expected observation item")
	}

	if item.Formality != FormalityInformal {
		t.Fatalf("formality = %d, want %d", item.Formality, FormalityInformal)
	}

	wantScope := []string{"internal/agent/cycle.go"}
	if !reflect.DeepEqual(item.ClaimScope, wantScope) {
		t.Fatalf("claim_scope = %v, want %v", item.ClaimScope, wantScope)
	}
}

func TestComputeFEff_MixedExplicitEvidence(t *testing.T) {
	chain := &EvidenceChain{
		Items: []EvidenceItem{
			NewEvidenceItem(ObservationTestPass, "go test ./internal/agent", 3),
			NewEvidenceItem(EvidenceMeasure, "measure ./internal/agent/cycle.go", 3),
			NewEvidenceItem(EvidenceAttached, "artifact ./internal/reff/reff.go", 3),
		},
	}

	got := ComputeFEff(chain)
	if got != FormalityStructuredInformal {
		t.Fatalf("F_eff = %d, want %d", got, FormalityStructuredInformal)
	}
}

func TestComputeFEff_EmptyAndObservationsOnly(t *testing.T) {
	if got := ComputeFEff(nil); got != 0 {
		t.Fatalf("F_eff(nil) = %d, want 0", got)
	}

	chain := &EvidenceChain{
		Items: []EvidenceItem{
			NewEvidenceItem(ObservationLintPass, "go vet ./...", 3),
			NewEvidenceItem(ObservationFileReview, "internal/agent/evidence.go", 3),
		},
	}

	if got := ComputeFEff(chain); got != 0 {
		t.Fatalf("F_eff(observations_only) = %d, want 0", got)
	}
}

func TestComputeGEff_DeduplicatesExplicitScopes(t *testing.T) {
	chain := &EvidenceChain{
		Items: []EvidenceItem{
			NewEvidenceItem(ObservationTestPass, "go test ./internal/agent", 3),
			NewEvidenceItem(EvidenceMeasure, "measure ./internal/agent/cycle.go ./internal/reff/reff.go", 3),
			NewEvidenceItem(EvidenceAttached, "attach ./internal/reff/reff.go", 3),
		},
	}

	got := ComputeGEff(chain)
	want := []string{"internal/agent/cycle.go", "internal/reff/reff.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("G_eff = %v, want %v", got, want)
	}
}

func TestComputeAssurance_CombinesFGR(t *testing.T) {
	chain := &EvidenceChain{
		Items: []EvidenceItem{
			NewEvidenceItem(ObservationTestPass, "go test ./internal/agent", 3),
			NewEvidenceItem(EvidenceMeasure, "measure ./internal/agent/cycle.go", 3),
			NewEvidenceItem(EvidenceAttached, "attach ./internal/reff/reff.go", 2),
		},
	}

	got := ComputeAssurance(chain)
	want := AssuranceTuple{
		F: FormalityStructuredInformal,
		G: []string{"internal/agent/cycle.go", "internal/reff/reff.go"},
		R: 0.6,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("assurance = %+v, want %+v", got, want)
	}
}
