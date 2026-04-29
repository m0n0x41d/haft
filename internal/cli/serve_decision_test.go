package cli

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintDecision_DecidePersistsPredictions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
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

func TestHandleQuintDecision_DecideUsesTaskContextInArtifactID(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "decide",
		"selected_title":   "Use gRPC",
		"why_selected":     "Serve-mode decide should pass task_context into the DecisionRecord ID.",
		"selection_policy": "Prefer a transport decision that remains traceable to the implementation task.",
		"counterargument":  "Filename context can be mistaken for the decision's semantic authority.",
		"weakest_link":     "The slug is metadata only and can go stale if the task changes.",
		"task_context":     "Task #4: API/CLI cleanup",
		"why_not_others": []map[string]any{{
			"variant": "REST",
			"reason":  "It does not exercise the optional DecisionRecord slug path.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"Decision IDs lose their random suffix."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^dec-\d{8}-task-4-api-cli-cleanup-[0-9a-f]{8}$`)
	if !pattern.MatchString(ref) {
		t.Fatalf("created ref = %q, want sanitized task_context slug before 8-hex suffix", ref)
	}

	decision, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.TaskContext != "task-4-api-cli-cleanup" {
		t.Fatalf("structured task_context = %q, want sanitized slug", fields.TaskContext)
	}
}

// TestHandleQuintDecision_DecideReturnsArtifactID verifies that the decide
// action returns the canonical artifact ID as the second return value. This
// closes the cross-project recall bug where the global index was keyed by
// selected_title (collision-prone) instead of the real DecisionRecord ID.
//
// Two decisions with the same selected_title in the same project must produce
// distinct IDs — otherwise the cross-project index silently overwrites the
// first decision's entry on the second decide call.
func TestHandleQuintDecision_DecideReturnsArtifactID(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	args := func() map[string]any {
		return map[string]any{
			"action":           "decide",
			"selected_title":   "Use Postgres",
			"why_selected":     "The team already operates Postgres at scale; default storage choice.",
			"selection_policy": "Prefer the storage system the on-call team already operates at scale.",
			"counterargument":  "Postgres at scale assumes operational maturity that may not hold for new services.",
			"weakest_link":     "Operational maturity in this team is the binding factor.",
			"why_not_others": []map[string]any{{
				"variant": "MySQL",
				"reason":  "Team has no production operational experience with MySQL at the required scale.",
			}},
			"rollback": map[string]any{
				"triggers": []string{"Operational load makes Postgres untenable."},
			},
		}
	}

	_, ref1, err := handleQuintDecision(ctx, store, haftDir, args())
	if err != nil {
		t.Fatalf("first decide: %v", err)
	}
	if ref1 == "" {
		t.Fatal("first decide returned empty createdRef; expected canonical artifact ID")
	}

	_, ref2, err := handleQuintDecision(ctx, store, haftDir, args())
	if err != nil {
		t.Fatalf("second decide: %v", err)
	}
	if ref2 == "" {
		t.Fatal("second decide returned empty createdRef; expected canonical artifact ID")
	}

	if ref1 == ref2 {
		t.Fatalf("two decisions with same selected_title produced identical IDs (%q); cross-project index would collide", ref1)
	}

	// Both refs must match real persisted artifacts.
	for _, ref := range []string{ref1, ref2} {
		a, err := store.Get(ctx, ref)
		if err != nil || a == nil {
			t.Fatalf("createdRef %q does not resolve to a stored artifact: %v", ref, err)
		}
		if a.Meta.Kind != artifact.KindDecisionRecord {
			t.Fatalf("createdRef %q resolved to %s, want DecisionRecord", ref, a.Meta.Kind)
		}
	}
}

// TestHandleQuintDecision_NonDecideActionsReturnEmptyRef verifies that actions
// other than "decide" do not return a createdRef. Cross-project indexing is
// only triggered for decide; other actions mutate or read existing artifacts.
func TestHandleQuintDecision_NonDecideActionsReturnEmptyRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Apply against missing decision returns plain-language stub, not error.
	_, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action": "apply",
	})
	if err != nil {
		t.Fatalf("apply with no decision: %v", err)
	}
	if ref != "" {
		t.Fatalf("apply returned createdRef %q; expected empty for non-creating action", ref)
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

	_, _, err = handleQuintDecision(ctx, store, haftDir, map[string]any{
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
