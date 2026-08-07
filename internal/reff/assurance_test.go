package reff

import (
	"errors"
	"testing"
	"time"
)

func TestComputeDecisionAssurance_ComputesPerClaimDecomposition(t *testing.T) {
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	got, err := ComputeDecisionAssurance(
		[]string{"claim-latency", "claim-correctness"},
		[]Evidence{
			{
				ClaimRefs:       []string{"claim-latency"},
				Type:            "benchmark",
				Verdict:         "supports",
				CongruenceLevel: 3,
				ValidUntil:      "2026-05-01T00:00:00Z",
			},
			{
				ClaimRefs:       []string{"claim-latency"},
				Type:            "documentation",
				Verdict:         "weakens",
				CongruenceLevel: 2,
				ValidUntil:      "2026-05-01T00:00:00Z",
			},
			{
				ClaimRefs:       []string{"claim-correctness"},
				Type:            "test_result",
				Verdict:         "accepted",
				CongruenceLevel: 3,
				ValidUntil:      "2026-05-01T00:00:00Z",
			},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	claims := claimAssuranceIndex(got.Claims)
	latency := claims["claim-latency"]
	correctness := claims["claim-correctness"]

	if latency.REff != 0.4 {
		t.Fatalf("latency R_eff = %.2f, want 0.40", latency.REff)
	}
	if latency.FEff != 1 {
		t.Fatalf("latency F_eff = %d, want 1", latency.FEff)
	}
	if latency.GEff != 2 {
		t.Fatalf("latency G_eff = %d, want 2", latency.GEff)
	}
	if correctness.REff != 1.0 {
		t.Fatalf("correctness R_eff = %.2f, want 1.00", correctness.REff)
	}
	if correctness.FEff != 2 {
		t.Fatalf("correctness F_eff = %d, want 2", correctness.FEff)
	}
	if correctness.GEff != 3 {
		t.Fatalf("correctness G_eff = %d, want 3", correctness.GEff)
	}
	if got.REff != 0.4 {
		t.Fatalf("decision R_eff = %.2f, want 0.40", got.REff)
	}
	if got.FEff != 1 {
		t.Fatalf("decision F_eff = %d, want 1", got.FEff)
	}
	if got.GEff != 2 {
		t.Fatalf("decision G_eff = %d, want 2", got.GEff)
	}
}

func TestDeriveFormality_UsesExplicitLevelBeforeType(t *testing.T) {
	got := DeriveFormality(Evidence{
		Type:           "documentation",
		FormalityLevel: 7,
		HasFormality:   true,
	})

	if got != 7 {
		t.Fatalf("DeriveFormality(explicit=7) = %d, want 7", got)
	}
}

func TestFormalityScaleNormalizationKeepsLegacyLossExplicit(t *testing.T) {
	current := NormalizeFormalityScale(FormalityScale{
		ScaleID: FormalityScaleCurrent,
		Level:   9,
	})
	if current.Level != 9 {
		t.Fatalf("current level = %d, want 9", current.Level)
	}

	legacy := NormalizeFormalityScale(FormalityScale{
		ScaleID: FormalityScaleLegacy,
		Level:   9,
	})
	if legacy.Level != 3 {
		t.Fatalf("legacy level = %d, want clamped F3", legacy.Level)
	}

	bridge := LegacyFormalityBridge(legacy.Level)
	if bridge.Loss != FormalityBridgeLegacyLoss {
		t.Fatalf("bridge loss = %q", bridge.Loss)
	}
}

func TestComputeDecisionAssurance_PreservesExplicitF7(t *testing.T) {
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	got, err := ComputeDecisionAssurance(
		[]string{"claim-machine-checkable"},
		[]Evidence{
			{
				ClaimRefs:       []string{"claim-machine-checkable"},
				Type:            "documentation",
				Verdict:         "supports",
				CongruenceLevel: 3,
				FormalityLevel:  7,
				HasFormality:    true,
				ValidUntil:      "2026-05-01T00:00:00Z",
			},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.FEff != 7 {
		t.Fatalf("decision F_eff = %d, want 7", got.FEff)
	}
	if got.Claims[0].FEff != 7 {
		t.Fatalf("claim F_eff = %d, want 7", got.Claims[0].FEff)
	}
}

func TestGroundednessFromCL(t *testing.T) {
	cases := []struct {
		name string
		cl   int
		want int
	}{
		{name: "same context", cl: 3, want: 3},
		{name: "similar context", cl: 2, want: 2},
		{name: "indirect context", cl: 1, want: 1},
		{name: "opposed context", cl: 0, want: 0},
		{name: "invalid context", cl: -1, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := GroundednessFromCL(tc.cl)
			if got != tc.want {
				t.Fatalf("GroundednessFromCL(%d) = %d, want %d", tc.cl, got, tc.want)
			}
		})
	}
}

func TestComputeDecisionAssurance_RejectsCL0Supports(t *testing.T) {
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	_, err := ComputeDecisionAssurance(
		[]string{"claim-latency"},
		[]Evidence{
			{
				ClaimRefs:       []string{"claim-latency"},
				Type:            "benchmark",
				Verdict:         "supports",
				CongruenceLevel: 0,
				ValidUntil:      "2026-05-01T00:00:00Z",
			},
		},
		now,
	)
	if !errors.Is(err, ErrInadmissibleEvidence) {
		t.Fatalf("error = %v, want ErrInadmissibleEvidence", err)
	}
}

func claimAssuranceIndex(values []ClaimAssurance) map[string]ClaimAssurance {
	index := make(map[string]ClaimAssurance, len(values))

	for _, value := range values {
		index[value.ClaimRef] = value
	}

	return index
}
