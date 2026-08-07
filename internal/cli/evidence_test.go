package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/reff"
)

func TestEvidencePathHelpNamesAuthorityBoundaries(t *testing.T) {
	t.Parallel()

	normalized := strings.ToLower(strings.Join(strings.Fields(evidencePathCmd.Long), " "))
	for _, want := range []string{"read-only", "does not create evidence", "approve", "gate", "claim truth", "global truth", "publication"} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("evidence path help missing %q:\n%s", want, evidencePathCmd.Long)
		}
	}
}

func TestWriteEvidencePathSummaryNamesCurrentFormalityScale(t *testing.T) {
	t.Parallel()

	scale := reff.CurrentFormalityScale(7)
	record := artifact.EvidencePathRecord{
		RecordKind:  artifact.EvidencePathRecordKind,
		Authority:   artifact.EvidencePathAuthority,
		ArtifactRef: "dec-1",
		Evidence: artifact.EvidencePathEvidence{
			ID:             "evid-1",
			FormalityLevel: 7,
			FormalityScale: &scale,
		},
		RelianceDisposition: artifact.RelianceDisposition{
			Disposition: artifact.EvidenceRelianceBounded,
			Reason:      "test",
		},
		ClaimBinding: artifact.EvidenceClaimBinding{
			Status: artifact.EvidenceClaimBindingBound,
		},
		TraceBinding: artifact.EvidenceTraceBinding{
			Status: artifact.EvidenceTraceBindingDeclared,
		},
		CurrentnessWindow: artifact.EvidenceCurrentnessWindow{
			Status: artifact.EvidenceCurrentnessCurrent,
		},
		AuthorityBoundary: artifact.EvidenceAuthorityBoundary{
			Approval:     artifact.EvidenceBoundaryNotApproval,
			GateDecision: artifact.EvidenceBoundaryNotGateDecision,
			ClaimTruth:   artifact.EvidenceBoundaryNotClaimTruth,
			GlobalTruth:  artifact.EvidenceBoundaryNotGlobalTruth,
			Publication:  artifact.EvidenceBoundaryNotPublication,
		},
	}

	var out bytes.Buffer
	if err := writeEvidencePathSummary(&out, record); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "formality: level=F7 scale=fpf-2026-f0-f9 bridge=none loss=none") {
		t.Fatalf("summary did not name current formality scale:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "authority_boundary: approval=not_approval gate_decision=not_gate_decision claim_truth=not_claim_truth global_truth=not_global_truth publication=not_publication") {
		t.Fatalf("summary did not name full authority boundary:\n%s", out.String())
	}
}

func TestWriteEvidencePathSummaryNamesLegacyFormalityBridge(t *testing.T) {
	t.Parallel()

	scale := reff.LegacyFormalityScale(2)
	bridge := reff.LegacyFormalityBridge(2)
	record := artifact.EvidencePathRecord{
		RecordKind:  artifact.EvidencePathRecordKind,
		Authority:   artifact.EvidencePathAuthority,
		ArtifactRef: "dec-1",
		Evidence: artifact.EvidencePathEvidence{
			ID:              "evid-1",
			FormalityLevel:  2,
			FormalityScale:  &scale,
			FormalityBridge: &bridge,
		},
		RelianceDisposition: artifact.RelianceDisposition{
			Disposition: artifact.EvidenceRelianceAdvisory,
			Reason:      "test",
		},
		ClaimBinding: artifact.EvidenceClaimBinding{
			Status: artifact.EvidenceClaimBindingNotRequested,
		},
		TraceBinding: artifact.EvidenceTraceBinding{
			Status: artifact.EvidenceTraceBindingMissing,
		},
		CurrentnessWindow: artifact.EvidenceCurrentnessWindow{
			Status: artifact.EvidenceCurrentnessPerpetual,
		},
		AuthorityBoundary: artifact.EvidenceAuthorityBoundary{
			Approval:     artifact.EvidenceBoundaryNotApproval,
			GateDecision: artifact.EvidenceBoundaryNotGateDecision,
			ClaimTruth:   artifact.EvidenceBoundaryNotClaimTruth,
			GlobalTruth:  artifact.EvidenceBoundaryNotGlobalTruth,
			Publication:  artifact.EvidenceBoundaryNotPublication,
		},
	}

	var out bytes.Buffer
	if err := writeEvidencePathSummary(&out, record); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "bridge=haft-legacy-f0-f3->fpf-2026-f0-f9") {
		t.Fatalf("summary did not name legacy formality bridge:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "loss=legacy-scale-has-fewer-buckets") {
		t.Fatalf("summary did not name legacy formality loss:\n%s", out.String())
	}
}

func TestWriteEvidencePathSummaryNamesFormalityDiagnostics(t *testing.T) {
	t.Parallel()

	scale := reff.UnversionedFormalityScale(2)
	bridge := reff.UnversionedFormalityBridge(2)
	record := artifact.EvidencePathRecord{
		RecordKind:  artifact.EvidencePathRecordKind,
		Authority:   artifact.EvidencePathAuthority,
		ArtifactRef: "dec-1",
		Evidence: artifact.EvidencePathEvidence{
			ID:                   "evid-1",
			FormalityLevel:       2,
			FormalityScale:       &scale,
			FormalityBridge:      &bridge,
			FormalityDiagnostics: []string{artifact.EvidenceFormalityDiagnosticUnversioned},
		},
		RelianceDisposition: artifact.RelianceDisposition{
			Disposition: artifact.EvidenceRelianceBlocked,
			Reason:      "current_formality_required",
		},
		ClaimBinding: artifact.EvidenceClaimBinding{
			Status: artifact.EvidenceClaimBindingBound,
		},
		TraceBinding: artifact.EvidenceTraceBinding{
			Status: artifact.EvidenceTraceBindingDeclared,
		},
		CurrentnessWindow: artifact.EvidenceCurrentnessWindow{
			Status: artifact.EvidenceCurrentnessCurrent,
		},
		AuthorityBoundary: artifact.EvidenceAuthorityBoundary{
			Approval:     artifact.EvidenceBoundaryNotApproval,
			GateDecision: artifact.EvidenceBoundaryNotGateDecision,
			ClaimTruth:   artifact.EvidenceBoundaryNotClaimTruth,
			GlobalTruth:  artifact.EvidenceBoundaryNotGlobalTruth,
			Publication:  artifact.EvidenceBoundaryNotPublication,
		},
	}

	var out bytes.Buffer
	if err := writeEvidencePathSummary(&out, record); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "formality_diagnostics: unversioned_formality_source_scale_missing") {
		t.Fatalf("summary did not name formality diagnostics:\n%s", out.String())
	}
}
