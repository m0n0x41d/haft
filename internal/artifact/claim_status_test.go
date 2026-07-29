package artifact

import (
	"encoding/json"
	"reflect"
	"strings"
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

func TestNewDecisionClaims_PreservesVerifyAfter(t *testing.T) {
	inputs := []PredictionInput{
		{
			Claim:       "Error rate drops 30%",
			Observable:  "grafana dashboard X",
			Threshold:   "< 2%",
			VerifyAfter: "2026-04-16",
		},
		{
			Claim:      "No latency regression",
			Observable: "wrk benchmark",
			Threshold:  "p99 < 50ms",
		},
	}

	got := newDecisionClaims(inputs)

	if len(got) != 2 {
		t.Fatalf("expected 2 claims, got %d", len(got))
	}
	if got[0].VerifyAfter != "2026-04-16" {
		t.Fatalf("claim-001 VerifyAfter = %q, want %q", got[0].VerifyAfter, "2026-04-16")
	}
	if got[1].VerifyAfter != "" {
		t.Fatalf("claim-002 VerifyAfter = %q, want empty", got[1].VerifyAfter)
	}
}

func TestNormalizeDecisionClaims_PreservesVerifyAfter(t *testing.T) {
	input := []DecisionClaim{
		{
			ID:          "claim-001",
			Claim:       "Error rate drops",
			Observable:  "dashboard",
			Threshold:   "< 2%",
			Status:      ClaimStatusUnverified,
			VerifyAfter: " 2026-05-01 ",
		},
	}

	got := normalizeDecisionClaims(input)

	if len(got) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(got))
	}
	if got[0].VerifyAfter != "2026-05-01" {
		t.Fatalf("VerifyAfter = %q, want %q (trimmed)", got[0].VerifyAfter, "2026-05-01")
	}
}

func TestDecisionPredictionsFromClaims_PreservesVerifyAfter(t *testing.T) {
	claims := []DecisionClaim{
		{
			ID:          "claim-001",
			Claim:       "Error rate drops",
			Observable:  "dashboard",
			Threshold:   "< 2%",
			Status:      ClaimStatusUnverified,
			VerifyAfter: "2026-05-01",
		},
	}

	got := decisionPredictionsFromClaims(claims)

	if len(got) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(got))
	}
	if got[0].VerifyAfter != "2026-05-01" {
		t.Fatalf("VerifyAfter = %q, want %q", got[0].VerifyAfter, "2026-05-01")
	}
}

func TestNormalizeDecisionClaims_PreservesClaimLifecycleFields(t *testing.T) {
	input := []DecisionClaim{
		{
			ID:                   " claim-lifecycle ",
			Claim:                " claim ",
			Observable:           " observable ",
			Threshold:            " threshold ",
			LifecycleStatus:      ClaimLifecycleSuperseded,
			SuccessorRef:         " dec-new#claim-2 ",
			RetiredReason:        " narrowed by successor ",
			GovernanceTargetRefs: []string{" api_contract:haft/status ", "api_contract:haft/status"},
		},
		{
			Claim:           "legacy active claim",
			LifecycleStatus: "unknown",
		},
	}

	got := normalizeDecisionClaims(input)
	if len(got) != 2 {
		t.Fatalf("claims = %#v, want two", got)
	}
	if got[0].ID != "claim-lifecycle" {
		t.Fatalf("id = %q", got[0].ID)
	}
	if got[0].LifecycleStatus != ClaimLifecycleSuperseded {
		t.Fatalf("lifecycle_status = %q", got[0].LifecycleStatus)
	}
	if got[0].SuccessorRef != "dec-new#claim-2" {
		t.Fatalf("successor_ref = %q", got[0].SuccessorRef)
	}
	if got[0].RetiredReason != "narrowed by successor" {
		t.Fatalf("retired_reason = %q", got[0].RetiredReason)
	}
	if len(got[0].GovernanceTargetRefs) != 1 || got[0].GovernanceTargetRefs[0] != "api_contract:haft/status" {
		t.Fatalf("governance_target_refs = %#v", got[0].GovernanceTargetRefs)
	}
	if got[1].LifecycleStatus != "" {
		t.Fatalf("legacy/unknown lifecycle should stay empty in storage, got %q", got[1].LifecycleStatus)
	}
	if EffectiveClaimLifecycleStatus(got[1]) != ClaimLifecycleActive {
		t.Fatalf("effective legacy lifecycle = %q, want active", EffectiveClaimLifecycleStatus(got[1]))
	}
}

func TestBuildClaimLifecycleSummaryCountsLegacyActiveAndTerminalClaims(t *testing.T) {
	summary := buildClaimLifecycleSummary([]DecisionClaim{
		{Claim: "legacy active", GovernanceTargetRefs: []string{"symbol:A"}},
		{Claim: "refresh", LifecycleStatus: ClaimLifecycleRefreshDue},
		{Claim: "superseded", LifecycleStatus: ClaimLifecycleSuperseded, GovernanceTargetRefs: []string{"symbol:B"}},
		{Claim: "deprecated", LifecycleStatus: ClaimLifecycleDeprecated, GovernanceTargetRefs: []string{"symbol:B"}},
	})
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Active != 1 || summary.RefreshDue != 1 || summary.Superseded != 1 || summary.Deprecated != 1 {
		t.Fatalf("summary counts = %#v", summary)
	}
	if len(summary.GovernanceTargetRefs) != 2 {
		t.Fatalf("governance target refs = %#v, want two unique refs", summary.GovernanceTargetRefs)
	}
}

func TestDecisionClaimsFromPredictions_PreservesVerifyAfter(t *testing.T) {
	predictions := []DecisionPrediction{
		{
			Claim:       "Error rate drops",
			Observable:  "dashboard",
			Threshold:   "< 2%",
			Status:      ClaimStatusUnverified,
			VerifyAfter: "2026-05-01",
		},
	}

	got := decisionClaimsFromPredictions(predictions)

	if len(got) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(got))
	}
	if got[0].VerifyAfter != "2026-05-01" {
		t.Fatalf("VerifyAfter = %q, want %q", got[0].VerifyAfter, "2026-05-01")
	}
}

func TestDecisionPredictionCompatibilityPreservesCommand(t *testing.T) {
	fields := DecisionFields{
		Predictions: []DecisionPrediction{
			{
				Claim:      "artifact tests stay green",
				Observable: "go test on artifact package",
				Threshold:  "exit 0",
				Command:    " go test ./internal/artifact/ ",
			},
		},
	}

	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "\"claims\"") {
		t.Fatalf("compatibility marshal should persist canonical claims:\n%s", encoded)
	}
	if !strings.Contains(string(encoded), "\"command\":\"go test ./internal/artifact/\"") {
		t.Fatalf("compatibility marshal lost command:\n%s", encoded)
	}

	var decoded DecisionFields
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Claims[0].Command; got != "go test ./internal/artifact/" {
		t.Fatalf("decoded claim command = %q", got)
	}
	if got := decoded.Predictions[0].Command; got != "go test ./internal/artifact/" {
		t.Fatalf("decoded compatibility prediction command = %q", got)
	}
}
