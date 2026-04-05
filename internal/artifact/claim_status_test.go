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

func TestAdjudicateDecisionClaims_PreservesUnmatchedStatus(t *testing.T) {
	claims := []DecisionClaim{
		{
			ID:         "claim-001",
			Claim:      "Latency stays under 50ms",
			Observable: "publish latency p99",
			Threshold:  "< 50ms",
			Status:     ClaimStatusSupported,
		},
		{
			ID:         "claim-002",
			Claim:      "Throughput stays above 100k events/sec",
			Observable: "throughput",
			Threshold:  "> 100k events/sec",
			Status:     ClaimStatusSupported,
		},
	}

	got := adjudicateDecisionClaims(
		claims,
		[]string{"claim-002"},
		nil,
		nil,
		[]string{"Throughput stays above 100k events/sec (observed: 87k events/sec)"},
		[]string{"Throughput stays above 100k events/sec"},
	)

	want := []DecisionClaim{
		{
			ID:         "claim-001",
			Claim:      "Latency stays under 50ms",
			Observable: "publish latency p99",
			Threshold:  "< 50ms",
			Status:     ClaimStatusSupported,
		},
		{
			ID:         "claim-002",
			Claim:      "Throughput stays above 100k events/sec",
			Observable: "throughput",
			Threshold:  "> 100k events/sec",
			Status:     ClaimStatusRefuted,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("adjudicateDecisionClaims() = %#v, want %#v", got, want)
	}
}

func TestRebuildDecisionClaimsFromEvidence_UsesActiveClaimEvidence(t *testing.T) {
	claims := []DecisionClaim{
		{
			ID:         "claim-001",
			Claim:      "Latency stays under 50ms",
			Observable: "publish latency p99",
			Threshold:  "< 50ms",
			Status:     ClaimStatusUnverified,
		},
		{
			ID:         "claim-002",
			Claim:      "Throughput stays above 100k events/sec",
			Observable: "throughput",
			Threshold:  "> 100k events/sec",
			Status:     ClaimStatusUnverified,
		},
	}

	activeEvidence := []EvidenceItem{
		{
			Type:      "benchmark",
			Verdict:   "supports",
			ClaimRefs: []string{"claim-001"},
		},
		{
			Type:      "measurement",
			Verdict:   "failed",
			ClaimRefs: []string{"claim-002"},
		},
	}

	got := rebuildDecisionClaimsFromEvidence(claims, activeEvidence)
	want := []DecisionClaim{
		{
			ID:         "claim-001",
			Claim:      "Latency stays under 50ms",
			Observable: "publish latency p99",
			Threshold:  "< 50ms",
			Status:     ClaimStatusSupported,
		},
		{
			ID:         "claim-002",
			Claim:      "Throughput stays above 100k events/sec",
			Observable: "throughput",
			Threshold:  "> 100k events/sec",
			Status:     ClaimStatusRefuted,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebuildDecisionClaimsFromEvidence() = %#v, want %#v", got, want)
	}
}

func TestMeasurementClaimEvidence_SplitsMixedMeasurementByClaim(t *testing.T) {
	claims := []DecisionClaim{
		{
			ID:         "claim-001",
			Claim:      "Latency stays under 50ms",
			Observable: "publish latency p99",
			Threshold:  "< 50ms",
		},
		{
			ID:         "claim-002",
			Claim:      "Throughput stays above 100k events/sec",
			Observable: "throughput",
			Threshold:  "> 100k events/sec",
		},
	}

	got := measurementClaimEvidence(
		claims,
		[]string{"publish latency p99 < 50ms (observed: 44ms)"},
		[]string{"publish latency p99 < 50ms"},
		[]string{"Throughput stays above 100k events/sec (observed: 87k events/sec)"},
		[]string{"Throughput stays above 100k events/sec"},
	)
	want := []EvidenceItem{
		{
			Type:       "measurement",
			Verdict:    "supports",
			ClaimRefs:  []string{"claim-001"},
			ClaimScope: []string{"Latency stays under 50ms"},
		},
		{
			Type:       "measurement",
			Verdict:    "refutes",
			ClaimRefs:  []string{"claim-002"},
			ClaimScope: []string{"Throughput stays above 100k events/sec"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("measurementClaimEvidence() = %#v, want %#v", got, want)
	}
}
