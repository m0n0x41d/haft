package present

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/graph"
)

func mkDecision(t *testing.T, claims []artifact.DecisionClaim) *artifact.Artifact {
	t.Helper()
	sd, err := json.Marshal(artifact.DecisionFields{Claims: claims})
	if err != nil {
		t.Fatal(err)
	}
	return &artifact.Artifact{StructuredData: string(sd)}
}

// TestDecisionVerificationTag is the trust-decay signal: a governing decision
// surfaces how many of its predictions remain unverified, so unchecked rationale
// does not read as authoritative.
func TestDecisionVerificationTag(t *testing.T) {
	mixed := mkDecision(t, []artifact.DecisionClaim{
		{Claim: "a", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusSupported},
		{Claim: "b", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusUnverified},
		{Claim: "c", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusUnverified},
	})
	if got := decisionVerificationTag(mixed); got != " · 2/3 predictions unverified" {
		t.Errorf("mixed = %q, want ' · 2/3 predictions unverified'", got)
	}

	allVerified := mkDecision(t, []artifact.DecisionClaim{
		{Claim: "a", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusSupported},
	})
	if got := decisionVerificationTag(allVerified); got != "" {
		t.Errorf("all-verified = %q, want empty", got)
	}

	if got := decisionVerificationTag(&artifact.Artifact{}); got != "" {
		t.Errorf("no-claims = %q, want empty", got)
	}
}

func TestCodeContextResponse_CompactsInvariantsAndFullRestoresThem(t *testing.T) {
	invariants := make([]graph.Invariant, 0, 20)
	for i := 1; i <= 20; i++ {
		invariants = append(invariants, graph.Invariant{
			Text:          fmt.Sprintf("invariant-%02d", i),
			DecisionTitle: "Context decision",
		})
	}
	cc := contextgraph.CodeContext{
		Target:     contextgraph.Target{File: "internal/x.go"},
		Invariants: invariants,
	}

	compact := CodeContextResponse(cc)
	if !strings.Contains(compact, "invariant-12") {
		t.Fatalf("compact response should include the capped visible prefix:\n%s", compact)
	}
	if strings.Contains(compact, "invariant-13") {
		t.Fatalf("compact response should omit invariant 13+:\n%s", compact)
	}
	if !strings.Contains(compact, "8 more omitted") {
		t.Fatalf("compact response should name omitted invariant count:\n%s", compact)
	}

	full := CodeContextResponseFull(cc)
	if !strings.Contains(full, "invariant-20") {
		t.Fatalf("full response should restore all invariants:\n%s", full)
	}
	if strings.Contains(full, "more omitted") {
		t.Fatalf("full response must not include compact omission marker:\n%s", full)
	}
}
