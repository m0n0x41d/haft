package artifact

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestClassifyEvidenceFreshnessKeepsDebtSeparateFromPolicy(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		input EvidenceFreshnessClassificationInput
		want  EvidenceFreshnessClass
	}{
		{
			name: "dated",
			input: EvidenceFreshnessClassificationInput{
				ValidUntil:          "2099-01-01",
				AssuranceApplicable: true,
			},
			want: EvidenceFreshnessDated,
		},
		{
			name: "expired",
			input: EvidenceFreshnessClassificationInput{
				ValidUntil:          "2020-01-01",
				AssuranceApplicable: true,
			},
			want: EvidenceFreshnessExpired,
		},
		{
			name: "explicit perpetual with rationale",
			input: EvidenceFreshnessClassificationInput{
				PerpetualRationale:  "Foundational historical observation remains relevant by definition.",
				AssuranceApplicable: true,
			},
			want: EvidenceFreshnessExplicitPerpetualWithReason,
		},
		{
			name: "legacy blank unknown",
			input: EvidenceFreshnessClassificationInput{
				AssuranceApplicable: true,
			},
			want: EvidenceFreshnessLegacyBlankUnknown,
		},
		{
			name:  "not assurance applicable",
			input: EvidenceFreshnessClassificationInput{},
			want:  EvidenceFreshnessNotAssuranceApplicable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classification := ClassifyEvidenceFreshness(test.input, now)
			if classification.Class != test.want {
				t.Fatalf("class = %q, want %q", classification.Class, test.want)
			}
			if classification.Authority !=
				EvidenceFreshnessDiagnosticAuthority ||
				classification.ScoringEffect != "none" ||
				classification.AdmissionEffect != "none" {
				t.Fatalf("classification crossed policy boundary: %#v", classification)
			}
		})
	}
}

func TestClassifyEvidenceFreshnessKeepsUnparseableCarrierVisible(t *testing.T) {
	classification := ClassifyEvidenceFreshness(
		EvidenceFreshnessClassificationInput{
			ValidUntil:          "not-a-date",
			AssuranceApplicable: true,
		},
		time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC),
	)
	if classification.Class != EvidenceFreshnessLegacyBlankUnknown ||
		classification.Diagnostic !=
			"valid_until_unparseable_under_current_date_contract" {
		t.Fatalf("unparseable carrier classification = %#v", classification)
	}
}

func TestBuildEvidenceFreshnessInventoryIsAvailableOnlyAfterCompleteScan(
	t *testing.T,
) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(
		ctx,
		`INSERT INTO evidence_items
		 (id, artifact_ref, type, content, verdict, valid_until, created_at)
		 VALUES ('evid-freshness', 'dec-freshness', 'test', 'fixture',
		 'supports', '2099-01-01', '2026-08-09T12:00:00Z')`,
	); err != nil {
		t.Fatalf("seed evidence item: %v", err)
	}

	inventory, err := BuildEvidenceFreshnessInventory(ctx, store, now)
	if err != nil {
		t.Fatalf("BuildEvidenceFreshnessInventory returned error: %v", err)
	}
	if inventory.Posture != EvidenceFreshnessInventoryPostureAvailable ||
		inventory.Diagnostic != "" ||
		inventory.TotalItems != 1 ||
		inventory.Dated != 1 {
		t.Fatalf("available inventory = %#v", inventory)
	}
}

func TestBuildEvidenceFreshnessInventoryFailureReturnsUnavailableWithoutPartialCounts(
	t *testing.T,
) {
	store := setupTestDB(t)
	if _, err := store.DB().Exec(`DROP TABLE evidence_items`); err != nil {
		t.Fatalf("drop evidence_items: %v", err)
	}
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

	inventory, err := BuildEvidenceFreshnessInventory(
		context.Background(),
		store,
		now,
	)
	if err == nil {
		t.Fatal("BuildEvidenceFreshnessInventory returned nil error")
	}
	if inventory.Posture != EvidenceFreshnessInventoryPostureUnavailable ||
		inventory.Authority != EvidenceFreshnessDiagnosticAuthority ||
		inventory.CheckedAt != now.Format(time.RFC3339) ||
		!strings.Contains(inventory.Diagnostic, "read evidence freshness carriers") {
		t.Fatalf("unavailable inventory identity = %#v", inventory)
	}
	if inventory.TotalItems != 0 ||
		inventory.Dated != 0 ||
		inventory.Expired != 0 ||
		inventory.ExplicitPerpetualWithRationale != 0 ||
		inventory.LegacyBlankUnknown != 0 ||
		inventory.NotAssuranceApplicable != 0 ||
		inventory.UnparseableValidUntil != 0 ||
		inventory.FindingsAdded != 0 ||
		inventory.LegacyBlankUnknownIsCCED1Violation ||
		inventory.ScoringChanged ||
		inventory.AdmissionChanged ||
		inventory.MutationsPerformed {
		t.Fatalf("unavailable inventory exposed results or effects: %#v", inventory)
	}
}

func TestBuildEvidenceFreshnessInventoryPreservesContextCancellation(t *testing.T) {
	store := setupTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inventory, err := BuildEvidenceFreshnessInventory(ctx, store, time.Time{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if inventory.Posture != EvidenceFreshnessInventoryPostureUnavailable ||
		!strings.Contains(inventory.Diagnostic, context.Canceled.Error()) {
		t.Fatalf("cancelled inventory = %#v", inventory)
	}
}
