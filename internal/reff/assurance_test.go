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

	if got != 2 {
		t.Fatalf("DeriveFormality(explicit=7) = %d, want 2", got)
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
