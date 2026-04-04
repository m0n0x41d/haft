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
