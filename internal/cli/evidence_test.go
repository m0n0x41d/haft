package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/reff"
)

func TestWriteEvidencePathSummaryNamesCurrentFormalityScale(t *testing.T) {
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
			GlobalTruth:  artifact.EvidenceBoundaryNotGlobalTruth,
		},
	}

	var out bytes.Buffer
	if err := writeEvidencePathSummary(&out, record); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "formality: level=F7 scale=fpf-2026-f0-f9 bridge=none loss=none") {
		t.Fatalf("summary did not name current formality scale:\n%s", out.String())
	}
}

func TestWriteEvidencePathSummaryNamesLegacyFormalityBridge(t *testing.T) {
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
			GlobalTruth:  artifact.EvidenceBoundaryNotGlobalTruth,
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
