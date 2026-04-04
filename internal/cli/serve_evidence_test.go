package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintDecision_EvidencePersistsValidUntil(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Keep attached evidence inspectable",
		WhySelected:     "Need a decision artifact for the evidence handler",
		SelectionPolicy: "Prefer the smallest decision artifact that still exercises the CLI evidence path against a real decision.",
		CounterArgument: "A synthetic decision record can miss coupling that appears in a real compare-driven decision.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Attach evidence to a note",
			Reason:  "This handler test explicitly needs a decision artifact target.",
		}},
		WeakestLink: "The decision is synthetic and therefore weaker than a real compared choice.",
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Evidence attachment stops preserving valid_until metadata"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	validUntil := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	result, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "evidence",
		"artifact_ref":     decision.Meta.ID,
		"evidence_content": "Load-test evidence remains valid through the current release window.",
		"evidence_type":    "benchmark",
		"evidence_verdict": "supports",
		"valid_until":      validUntil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Evidence attached:") {
		t.Fatalf("unexpected response: %s", result)
	}

	items, err := store.GetEvidenceItems(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(items))
	}
	if items[0].ValidUntil != validUntil {
		t.Fatalf("valid_until = %q, want %q", items[0].ValidUntil, validUntil)
	}
}
