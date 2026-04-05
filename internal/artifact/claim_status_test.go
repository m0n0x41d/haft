package artifact

import (
	"reflect"
	"testing"
)

func TestNewDecisionClaims_InitializesCanonicalRuntimeState(t *testing.T) {
	inputs := []PredictionInput{
		{
			Claim:      "  Latency stays under 50ms  ",
			Observable: " publish latency p99 ",
			Threshold:  " < 50ms ",
		},
		{},
	}

	got := newDecisionClaims(inputs)
	want := []DecisionClaim{{
		ID:         "claim-001",
		Claim:      "Latency stays under 50ms",
		Observable: "publish latency p99",
		Threshold:  "< 50ms",
		Status:     ClaimStatusUnverified,
	}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("newDecisionClaims() = %#v, want %#v", got, want)
	}

	compatibility := decisionPredictionsFromClaims(got)
	wantCompatibility := []DecisionPrediction{{
		Claim:      "Latency stays under 50ms",
		Observable: "publish latency p99",
		Threshold:  "< 50ms",
		Status:     ClaimStatusUnverified,
	}}

	if !reflect.DeepEqual(compatibility, wantCompatibility) {
		t.Fatalf("decisionPredictionsFromClaims() = %#v, want %#v", compatibility, wantCompatibility)
	}
}

func TestClaimStatusFromPredictionMeasureMatch(t *testing.T) {
	cases := []struct {
		name  string
		match PredictionMeasureMatch
		want  ClaimStatus
	}{
		{
			name:  "no measurement keeps claim unverified",
			match: PredictionMeasureMatch{},
			want:  ClaimStatusUnverified,
		},
		{
			name: "measurement without direct match is inconclusive",
			match: PredictionMeasureMatch{
				MeasurementRecorded: true,
			},
			want: ClaimStatusInconclusive,
		},
		{
			name: "matched met criterion supports claim",
			match: PredictionMeasureMatch{
				MeasurementRecorded: true,
				CriteriaMet:         true,
			},
			want: ClaimStatusSupported,
		},
		{
			name: "matched unmet criterion refutes claim",
			match: PredictionMeasureMatch{
				MeasurementRecorded: true,
				CriteriaNotMet:      true,
			},
			want: ClaimStatusRefuted,
		},
		{
			name: "contradictory matches weaken claim",
			match: PredictionMeasureMatch{
				MeasurementRecorded: true,
				CriteriaMet:         true,
				CriteriaNotMet:      true,
			},
			want: ClaimStatusWeakened,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			got := ClaimStatusFromPredictionMeasureMatch(tc.match)

			if got != tc.want {
				t.Fatalf("ClaimStatusFromPredictionMeasureMatch() = %q, want %q", got, tc.want)
			}
		})
	}
}
