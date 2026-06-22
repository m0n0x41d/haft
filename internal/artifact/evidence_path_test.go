package artifact

import (
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

func TestBuildEvidencePathRecordAllowsOnlyBoundedReliance(t *testing.T) {
	record := BuildEvidencePathRecord(
		EvidencePathInput{
			ArtifactRef:  "dec-1",
			EvidenceRef:  "evid-1",
			ClaimRef:     "claim-1",
			AttemptedUse: "commission preflight",
			ProducerRef:  "agent:local",
			MethodRef:    "mpull-1",
			WorkRef:      "wc-1",
		},
		evidencePathItem(),
		evidencePathNow(),
	)

	if record.RelianceDisposition.Disposition != EvidenceRelianceBounded {
		t.Fatalf("reliance = %+v, want bounded", record.RelianceDisposition)
	}
	if record.AuthorityBoundary.Approval != EvidenceBoundaryNotApproval {
		t.Fatalf("authority_boundary = %+v, want not approval", record.AuthorityBoundary)
	}
	if record.AuthorityBoundary.GateDecision != EvidenceBoundaryNotGateDecision {
		t.Fatalf("authority_boundary = %+v, want not gate", record.AuthorityBoundary)
	}
	if record.AuthorityBoundary.GlobalTruth != EvidenceBoundaryNotGlobalTruth {
		t.Fatalf("authority_boundary = %+v, want not global truth", record.AuthorityBoundary)
	}
}

func TestBuildEvidencePathRecordKeepsFormalitySeparateFromAuthority(t *testing.T) {
	item := evidencePathItem()
	scale := reff.CurrentFormalityScale(9)
	item.FormalityLevel = 9
	item.FormalityScale = &scale

	record := BuildEvidencePathRecord(
		EvidencePathInput{
			ArtifactRef:  "dec-1",
			EvidenceRef:  "evid-1",
			ClaimRef:     "claim-1",
			AttemptedUse: "commission preflight",
			ProducerRef:  "agent:local",
			MethodRef:    "mpull-1",
			WorkRef:      "wc-1",
		},
		item,
		evidencePathNow(),
	)

	if record.Evidence.FormalityScale == nil {
		t.Fatal("formality scale missing")
	}
	if record.Evidence.FormalityScale.ScaleID != reff.FormalityScaleCurrent {
		t.Fatalf("scale = %q", record.Evidence.FormalityScale.ScaleID)
	}
	if record.AuthorityBoundary.Approval != EvidenceBoundaryNotApproval {
		t.Fatalf("approval boundary = %q", record.AuthorityBoundary.Approval)
	}
	if record.AuthorityBoundary.GateDecision != EvidenceBoundaryNotGateDecision {
		t.Fatalf("gate boundary = %q", record.AuthorityBoundary.GateDecision)
	}
	if record.AuthorityBoundary.GlobalTruth != EvidenceBoundaryNotGlobalTruth {
		t.Fatalf("global truth boundary = %q", record.AuthorityBoundary.GlobalTruth)
	}
}

func TestBuildEvidencePathRecordBlocksLegacyFormalityWhenCurrentRequired(t *testing.T) {
	item := evidencePathItem()
	scale := reff.LegacyFormalityScale(2)
	bridge := reff.LegacyFormalityBridge(2)
	item.FormalityScale = &scale
	item.FormalityBridge = &bridge

	record := BuildEvidencePathRecord(
		EvidencePathInput{
			ArtifactRef:              "dec-1",
			EvidenceRef:              "evid-1",
			ClaimRef:                 "claim-1",
			AttemptedUse:             "release gate reliance",
			RequiresCurrentFormality: true,
			ProducerRef:              "agent:local",
			MethodRef:                "mpull-1",
			WorkRef:                  "wc-1",
		},
		item,
		evidencePathNow(),
	)

	if record.AttemptedUse.RequiresCurrentFormality != true {
		t.Fatalf("attempted_use = %+v, want current formality requirement", record.AttemptedUse)
	}
	if record.RelianceDisposition.Disposition != EvidenceRelianceBlocked {
		t.Fatalf("reliance = %+v, want blocked", record.RelianceDisposition)
	}
	if record.RelianceDisposition.Reason != "current_formality_required" {
		t.Fatalf("reason = %q, want current_formality_required", record.RelianceDisposition.Reason)
	}
	if record.AuthorityBoundary.Approval != EvidenceBoundaryNotApproval {
		t.Fatalf("approval boundary = %q", record.AuthorityBoundary.Approval)
	}
	if record.AuthorityBoundary.GateDecision != EvidenceBoundaryNotGateDecision {
		t.Fatalf("gate boundary = %q", record.AuthorityBoundary.GateDecision)
	}
	if record.AuthorityBoundary.GlobalTruth != EvidenceBoundaryNotGlobalTruth {
		t.Fatalf("global truth boundary = %q", record.AuthorityBoundary.GlobalTruth)
	}
}

func TestBuildEvidencePathRecordBlocksMissingAttemptedUse(t *testing.T) {
	record := BuildEvidencePathRecord(
		EvidencePathInput{
			ArtifactRef: "dec-1",
			EvidenceRef: "evid-1",
			ClaimRef:    "claim-1",
			MethodRef:   "mpull-1",
		},
		evidencePathItem(),
		evidencePathNow(),
	)

	if record.RelianceDisposition.Disposition != EvidenceRelianceBlocked {
		t.Fatalf("reliance = %+v, want blocked", record.RelianceDisposition)
	}
	if record.RelianceDisposition.Reason != "attempted_use_required" {
		t.Fatalf("reason = %q, want attempted_use_required", record.RelianceDisposition.Reason)
	}
}

func TestBuildEvidencePathRecordBlocksExpiredEvidence(t *testing.T) {
	item := evidencePathItem()
	item.ValidUntil = "2020-01-01"

	record := BuildEvidencePathRecord(
		EvidencePathInput{
			ArtifactRef:  "dec-1",
			EvidenceRef:  "evid-1",
			ClaimRef:     "claim-1",
			AttemptedUse: "commission preflight",
			MethodRef:    "mpull-1",
		},
		item,
		evidencePathNow(),
	)

	if record.CurrentnessWindow.Status != EvidenceCurrentnessExpired {
		t.Fatalf("currentness = %+v, want expired", record.CurrentnessWindow)
	}
	if record.RelianceDisposition.Reason != "evidence_not_current" {
		t.Fatalf("reason = %q, want evidence_not_current", record.RelianceDisposition.Reason)
	}
}

func TestBuildEvidencePathRecordBlocksUnboundClaim(t *testing.T) {
	record := BuildEvidencePathRecord(
		EvidencePathInput{
			ArtifactRef:  "dec-1",
			EvidenceRef:  "evid-1",
			ClaimRef:     "claim-404",
			AttemptedUse: "commission preflight",
			MethodRef:    "mpull-1",
		},
		evidencePathItem(),
		evidencePathNow(),
	)

	if record.ClaimBinding.Status != EvidenceClaimBindingNotBound {
		t.Fatalf("claim_binding = %+v, want not_bound", record.ClaimBinding)
	}
	if record.RelianceDisposition.Reason != "claim_not_bound_to_evidence" {
		t.Fatalf("reason = %q, want claim_not_bound_to_evidence", record.RelianceDisposition.Reason)
	}
}

func evidencePathItem() EvidenceItem {
	return EvidenceItem{
		ID:              "evid-1",
		Type:            "test",
		Verdict:         "supports",
		CarrierRef:      "local:test",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ClaimRefs:       []string{"claim-1"},
		ClaimScope:      []string{"acceptance-1"},
		ValidUntil:      "2099-01-01",
		Provenance:      "machine",
	}
}

func evidencePathNow() time.Time {
	return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
}
