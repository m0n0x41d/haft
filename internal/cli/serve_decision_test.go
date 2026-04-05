package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintDecision_DecidePersistsPredictions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "decide",
		"selected_title":   "gRPC",
		"why_selected":     "Plugin-mode decide should persist falsifiable predictions through the serve path.",
		"selection_policy": "Prefer the lowest latency option that stays inside the operational budget.",
		"counterargument":  "The simplified benchmark can miss production load variance.",
		"weakest_link":     "Operational confidence still depends on limited production-grade evidence.",
		"why_not_others": []map[string]any{{
			"variant": "REST",
			"reason":  "Higher steady-state latency with no decisive compensating advantage.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"Latency regresses in production."},
		},
		"predictions": []map[string]any{{
			"claim":      "Throughput stays above 100k events/sec",
			"observable": "throughput",
			"threshold":  "> 100k events/sec",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	decisions, err := store.ListByKind(ctx, artifact.KindDecisionRecord, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	decision, err := store.Get(ctx, decisions[0].Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if len(fields.Predictions) != 1 {
		t.Fatalf("expected 1 prediction, got %+v", fields.Predictions)
	}

	prediction := fields.Predictions[0]
	if prediction.Claim != "Throughput stays above 100k events/sec" {
		t.Fatalf("prediction claim = %q", prediction.Claim)
	}
	if prediction.Observable != "throughput" {
		t.Fatalf("prediction observable = %q", prediction.Observable)
	}
	if prediction.Threshold != "> 100k events/sec" {
		t.Fatalf("prediction threshold = %q", prediction.Threshold)
	}
}

func TestHandleQuintDecision_MeasureRejectsMalformedMeasurements(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Keep measurement parsing strict",
		WhySelected:     "Serve-mode measure should reject malformed arrays instead of truncating them.",
		SelectionPolicy: "Prefer payload validation that preserves semantic parity with the direct tool path.",
		CounterArgument: "Strict parsing can reject callers that relied on historical truncation.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Lenient truncation",
			Reason:  "Silently losing measured values corrupts the decision record.",
		}},
		WeakestLink: "Broken clients may need a migration step before relying on strict validation.",
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Plugin payloads become incompatible with the serve handler."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":       "measure",
		"decision_ref": decision.Meta.ID,
		"findings":     "The payload mixed strings and numbers.",
		"measurements": []any{"p99 latency: 18ms", 42},
		"verdict":      "partial",
	})
	if err == nil {
		t.Fatal("expected malformed measurements to be rejected")
	}
	if !strings.Contains(err.Error(), "measurements must be an array of strings") {
		t.Fatalf("unexpected error: %v", err)
	}
}
