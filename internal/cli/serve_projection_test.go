package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQuery_ProjectionRendersAuditView(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "gRPC",
		WhySelected:     "Lower latency with acceptable complexity.",
		SelectionPolicy: "Prefer the lowest latency option that stays inside the operational budget.",
		CounterArgument: "Tooling and local debugging remain weaker than the simpler HTTP baseline.",
		WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "REST", Reason: "Higher steady-state latency with no decisive cost advantage."}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"Latency regresses in production."}},
		ValidUntil:      "2026-12-31T00:00:00Z",
		Context:         "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef: decision.Meta.ID,
		Content:     "Replay benchmark kept p95 latency below 25ms.",
		Type:        "measurement",
		Verdict:     "supports",
		ClaimScope:  []string{"latency"},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, haftDir, map[string]any{
		"action":  "projection",
		"view":    "audit",
		"context": "payments",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	required := []string{
		"## Audit/Evidence View",
		"Selection policy: Prefer the lowest latency option that stays inside the operational budget.",
		"Evidence:",
		"── Haft",
	}

	for _, want := range required {
		if !strings.Contains(result, want) {
			t.Fatalf("projection response missing %q:\n%s", want, result)
		}
	}
}

func TestHandleQuintQuery_ProjectionRendersDelegatedBriefAlias(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
		Context:    "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "gRPC",
		WhySelected:     "Delegated handoff should stay tied to canonical state.",
		SelectionPolicy: "Prefer the lowest latency option that stays inside the operational budget.",
		CounterArgument: "Tooling and local debugging remain weaker than the simpler HTTP baseline.",
		WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "REST", Reason: "Higher steady-state latency with no decisive cost advantage."}},
		Invariants:      []string{"p99 latency remains below 50ms during cutover"},
		Admissibility:   []string{"No silent message loss during protocol migration"},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"Latency regresses in production."}},
		AffectedFiles:   []string{"internal/transport/grpc.go", "internal/transport/contracts.proto"},
		Predictions: []artifact.PredictionInput{
			{
				Claim:      "Throughput stays above 100k events/sec",
				Observable: "throughput",
				Threshold:  "> 100k events/sec",
			},
		},
		Context: "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, haftDir, map[string]any{
		"action":  "projection",
		"view":    "handoff",
		"context": "payments",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	required := []string{
		"## Delegated-Agent Brief",
		"Selected decision: gRPC",
		"Affected files: internal/transport/contracts.proto, internal/transport/grpc.go",
		"Invariants: p99 latency remains below 50ms during cutover",
		"Admissibility: No silent message loss during protocol migration",
		"Rollback triggers: Latency regresses in production.",
		"unverified: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
		"── Haft",
	}

	for _, want := range required {
		if !strings.Contains(result, want) {
			t.Fatalf("projection response missing %q:\n%s", want, result)
		}
	}
}

func TestHandleQuintQuery_ProjectionRendersChangeRationaleAlias(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
		Context:    "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "gRPC",
		WhySelected:     "It meets the latency target with acceptable operating cost.",
		SelectionPolicy: "Prefer the lowest latency option that stays inside the operational budget.",
		CounterArgument: "Tooling and local debugging remain weaker than the simpler HTTP baseline.",
		WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "REST", Reason: "Higher steady-state latency with no decisive cost advantage."}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"Latency regresses in production."}},
		Context:         "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = artifact.Measure(ctx, store, haftDir, artifact.MeasureInput{
		DecisionRef: decision.Meta.ID,
		Findings:    "Latency passed, rollout is still partially blocked on throughput headroom.",
		CriteriaMet: []string{
			"publish latency p99 < 50ms (observed: 44ms)",
		},
		CriteriaNotMet: []string{
			"Throughput stays above 100k events/sec (observed: 87k events/sec)",
		},
		Verdict: "partial",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, haftDir, map[string]any{
		"action":  "projection",
		"view":    "pr",
		"context": "payments",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	required := []string{
		"## PR/Change Rationale",
		"Selected change: gRPC",
		"Problem signal: Latency variance between protocols",
		"Selected variant: gRPC",
		"Why selected: It meets the latency target with acceptable operating cost.",
		"Rejected alternatives:",
		"- REST: Higher steady-state latency with no decisive cost advantage.",
		"Rollback summary: Latency regresses in production.",
		"Latest measurement verdict: partial",
		"── Haft",
	}

	for _, want := range required {
		if !strings.Contains(result, want) {
			t.Fatalf("projection response missing %q:\n%s", want, result)
		}
	}
}
