package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestWriteCorrespondenceGraphSummaryNamesCompleteAuthorityBoundary(t *testing.T) {
	record := artifact.QualifiedCorrespondenceGraph{
		RecordKind:  artifact.CorrespondenceGraphRecordKind,
		Authority:   artifact.CorrespondenceGraphAuthority,
		GraphRef:    "correspondence:dec-1",
		DecisionRef: "dec-1",
		PathStatus:  artifact.CorrespondencePathNotProof,
		AuthorityBoundary: artifact.CorrespondenceGraphBoundary{
			Proof:        artifact.CorrespondenceBoundaryNotProof,
			Evidence:     artifact.CorrespondenceBoundaryNotEvidence,
			Approval:     artifact.CorrespondenceBoundaryNotApproval,
			GateDecision: artifact.CorrespondenceBoundaryNotGateDecision,
			ClaimTruth:   artifact.CorrespondenceBoundaryNotClaimTruth,
			GlobalTruth:  artifact.CorrespondenceBoundaryNotGlobalTruth,
			Publication:  artifact.CorrespondenceBoundaryNotPublication,
		},
	}

	var out bytes.Buffer
	if err := writeCorrespondenceGraphSummary(&out, record); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "authority_boundary: proof=not_proof evidence=not_evidence approval=not_approval gate_decision=not_gate_decision claim_truth=not_claim_truth global_truth=not_global_truth publication=not_publication") {
		t.Fatalf("summary should name complete authority boundary:\n%s", got)
	}
}

func TestWriteEngineeringChangeCaseSummaryNamesCompleteAuthorityBoundary(t *testing.T) {
	record := artifact.EngineeringChangeCase{
		RecordKind:           artifact.EngineeringChangeCaseRecordKind,
		Authority:            artifact.EngineeringChangeCaseAuthority,
		CaseRef:              "change-case:dec-1",
		DecisionSpeechActRef: "dec-1",
		AuthorityBoundary: artifact.EngineeringChangeCaseBoundary{
			Proof:          artifact.EngineeringChangeCaseBoundaryNotProof,
			Approval:       artifact.EngineeringChangeCaseBoundaryNotApproval,
			GateDecision:   artifact.EngineeringChangeCaseBoundaryNotGateDecision,
			WorkOccurrence: artifact.EngineeringChangeCaseBoundaryNotWorkOccurrence,
			ClaimTruth:     artifact.EngineeringChangeCaseBoundaryNotClaimTruth,
			GlobalTruth:    artifact.EngineeringChangeCaseBoundaryNotGlobalTruth,
			Publication:    artifact.EngineeringChangeCaseBoundaryNotPublication,
		},
	}

	var out bytes.Buffer
	if err := writeEngineeringChangeCaseSummary(&out, record); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "authority_boundary: proof=not_proof approval=not_approval gate_decision=not_gate_decision work_occurrence=not_work_occurrence claim_truth=not_claim_truth global_truth=not_global_truth publication=not_publication") {
		t.Fatalf("summary should name complete authority boundary:\n%s", got)
	}
}
