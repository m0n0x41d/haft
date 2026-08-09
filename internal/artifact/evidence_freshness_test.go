package artifact

import (
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
