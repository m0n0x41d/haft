package artifact

import (
	"context"
	"testing"
)

func TestDeriveDecisionHealthDistinguishesNoEvidenceFromReadFailure(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	withoutEvidence := DeriveDecisionHealth(ctx, store, "dec-20260711-health")
	if withoutEvidence.EvidenceState != DecisionEvidenceNoActiveEvidence {
		t.Fatalf("evidence state = %q, want %q", withoutEvidence.EvidenceState, DecisionEvidenceNoActiveEvidence)
	}

	if _, err := store.DB().ExecContext(ctx, `DROP TABLE evidence_items`); err != nil {
		t.Fatal(err)
	}
	unavailable := DeriveDecisionHealth(ctx, store, "dec-20260711-health")
	if unavailable.EvidenceState != DecisionEvidenceUnavailable {
		t.Fatalf("evidence state = %q, want %q", unavailable.EvidenceState, DecisionEvidenceUnavailable)
	}
}

func TestDeriveDecisionVerificationSummaryExcludesTerminalClaims(t *testing.T) {
	decision := &Artifact{
		StructuredData: `{"claims":[
			{"id":"active","claim":"active","observable":"observe","threshold":"pass","status":"unverified","verify_after":"2026-08-03"},
			{"id":"refresh","claim":"refresh","observable":"observe","threshold":"pass","status":"unverified","lifecycle_status":"refresh_due","verify_after":"2026-08-02T12:00:00Z"},
			{"id":"supported","claim":"supported","observable":"observe","threshold":"pass","status":"supported","verify_after":"2026-08-01"},
			{"id":"superseded","claim":"superseded","observable":"observe","threshold":"pass","status":"unverified","lifecycle_status":"superseded","verify_after":"2026-07-01"},
			{"id":"deprecated","claim":"deprecated","observable":"observe","threshold":"pass","status":"unverified","lifecycle_status":"deprecated","verify_after":"2026-06-01"}
		]}`,
	}

	summary := DeriveDecisionVerificationSummary(decision)
	if summary.ActiveClaims != 3 || summary.UnverifiedClaims != 2 {
		t.Fatalf("summary = %#v, want 3 active and 2 unverified", summary)
	}
	if summary.NextScheduledCheck != "2026-08-02" {
		t.Fatalf("next scheduled check = %q, want 2026-08-02", summary.NextScheduledCheck)
	}
}

func TestDeriveDecisionVerificationSummaryZeroClaimHasNoScheduledCheck(t *testing.T) {
	summary := DeriveDecisionVerificationSummary(&Artifact{})
	if summary.ActiveClaims != 0 || summary.UnverifiedClaims != 0 || summary.NextScheduledCheck != "" {
		t.Fatalf("zero-claim summary = %#v", summary)
	}
}
