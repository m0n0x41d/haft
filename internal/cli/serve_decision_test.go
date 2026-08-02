package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// The serve/MCP surface receives proposal content, not a human approval
// request provenance. Rich decision fields are covered at the artifact core
// and host-routed CLI seam; this test keeps the transport boundary honest and proves that no
// model-supplied field can bypass it.
func TestHandleQuintDecision_DecideTreatsModelPayloadAsProposalOnly(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	out, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":            "decide",
		"problem_statement": "A rich model-authored proposal must not institute a DecisionRecord.",
		"selected_title":    "Use the host-routed decision path",
		"why_selected":      "The binding effect belongs to a host route with operator provenance.",
		"selection_policy":  "Reject every model or MCP attempt to institute the decision.",
		"counterargument":   "The proposal contains all fields needed by a valid decision.",
		"weakest_link":      "A future refactor could accidentally resume parsing before the authority check.",
		"task_context":      "Task #4: authority boundary",
		"affected_files":    []any{"internal/cli/serve.go"},
		"why_not_others": []map[string]any{{
			"variant": "Bind directly from MCP",
			"reason":  "Model-supplied arguments are not operator authorization.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"The manual binding surface becomes unavailable."},
		},
		"predictions": []map[string]any{{
			"claim":      "No DecisionRecord is persisted",
			"observable": "DecisionRecord count",
			"threshold":  "equals zero",
		}},
		"choice_result": map[string]any{
			"subject_ref":      "operator",
			"option_set":       []any{"Use the host-routed decision path", "Bind directly from MCP"},
			"comparison_basis": []any{"host conversation has operator request provenance", "MCP cannot observe the conversation"},
			"choice_rule":      "Require one direct unambiguous operator request.",
			"next_move":        string(artifact.ChoiceNextMoveChooseNow),
			"variant_ref":      "Use the host-routed decision path",
		},
		"transformation_record": map[string]any{
			"transformed_entity": "DecisionRecord authority boundary",
			"initial_state":      "model proposal",
			"post_state":         "host-routed operator-requested decision",
			"relation":           "requires",
			"context":            "operator request provenance",
		},
	})
	assertDecisionBindingUnavailable(t, err)
	if out != "" || ref != "" {
		t.Fatalf("rejected model binding returned output/ref %q/%q", out, ref)
	}
	for _, marker := range []string{
		"haft artifact create decision.decide --input-file",
		"host_routed_operator_request",
	} {
		if !strings.Contains(err.Error(), marker) {
			t.Fatalf("authority error omitted %q: %v", marker, err)
		}
	}

	decisions, listErr := store.ListByKind(ctx, artifact.KindDecisionRecord, 10)
	if listErr != nil {
		t.Fatalf("list decisions after rejected model binding: %v", listErr)
	}
	if len(decisions) != 0 {
		t.Fatalf("rejected model binding persisted %d DecisionRecord(s)", len(decisions))
	}
}

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

// TestHandleQuintDecision_BaselineAcceptsArtifactRef regression-tests issue #77:
// LLM clients naturally pass artifact_ref (the universal key in haft_refresh and
// the only documented ref for evidence). Before the fix, baseline/measure silently
// ignored artifact_ref and fell through to ListByKind auto-detect, landing on the
// wrong decision. Now both keys are accepted and the auto-detect is gone.
func TestHandleQuintDecision_BaselineAcceptsArtifactRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")

	older, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Older decision — the intended target",
		ProblemStatement: "Baseline routing must use the caller's explicit decision reference.",
		WhySelected:      "This is the decision we want to baseline by passing artifact_ref.",
		SelectionPolicy:  "Prefer explicit identification over implicit recency.",
		CounterArgument:  "Auto-detect may seem convenient but corrupts the artifact graph.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "auto-detect", Reason: "silent misrouting"}},
		WeakestLink:      "LLM clients may still confuse parameter names — schema docs must be clear.",
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Newer decision exists after — this is what ListByKind(...,1) would pick.
	_, _, err = artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Newer decision — should NOT be picked",
		ProblemStatement: "Baseline routing must not substitute a newer decision for an explicit target.",
		WhySelected:      "If auto-detect leaks, this newer decision would steal the baseline.",
		SelectionPolicy:  "Most-recent default is the trap we're closing.",
		CounterArgument:  "None.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "older decision", Reason: "test fixture"}},
		WeakestLink:      "None.",
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Touch a file so baseline has something to hash.
	tmpFile := filepath.Join(projectRoot, "sample.go")
	if werr := os.WriteFile(tmpFile, []byte("package sample\n"), 0o644); werr != nil {
		t.Fatal(werr)
	}

	out, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":         "baseline",
		"artifact_ref":   older.Meta.ID,
		"affected_files": []any{"sample.go"},
	})
	if err != nil {
		t.Fatalf("baseline with artifact_ref: %v", err)
	}
	if !strings.Contains(out, older.Meta.ID) {
		t.Fatalf("baseline response %q does not mention intended target %q", out, older.Meta.ID)
	}
}

// TestHandleQuintDecision_BaselineRequiresRef verifies that the silent auto-detect
// fall-through (issue #77) is gone — calling baseline without any ref errors loudly
// instead of picking the most-recent decision.
func TestHandleQuintDecision_BaselineRequiresRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Some decision",
		ProblemStatement: "Baseline without an explicit decision reference must fail closed.",
		WhySelected:      "Exists so the bug-prone auto-detect would have something to grab.",
		SelectionPolicy:  "Explicit refs only.",
		CounterArgument:  "None.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "implicit", Reason: "unsafe"}},
		WeakestLink:      "None.",
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action": "baseline",
	})
	if err != nil {
		t.Fatalf("baseline without ref should return guidance, not error: %v", err)
	}
	if !strings.Contains(out, "requires decision_ref") {
		t.Fatalf("baseline without ref should ask for decision_ref; got: %q", out)
	}
}

// TestHandleQuintDecision_MeasureAcceptsArtifactRef — same fix on the measure side.
func TestHandleQuintDecision_MeasureAcceptsArtifactRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	older, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Older decision — measure target",
		ProblemStatement: "Measurement routing must use the caller's explicit decision reference.",
		WhySelected:      "We want measure() to land here when artifact_ref names it.",
		SelectionPolicy:  "Explicit refs only.",
		CounterArgument:  "None.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "auto-detect", Reason: "corrupts graph"}},
		WeakestLink:      "None.",
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Newer decision — must not be picked",
		ProblemStatement: "Measurement routing must not substitute a newer decision for an explicit target.",
		WhySelected:      "Sanity guard for the regression test.",
		SelectionPolicy:  "Explicit refs only.",
		CounterArgument:  "None.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "older", Reason: "fixture"}},
		WeakestLink:      "None.",
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":       "measure",
		"artifact_ref": older.Meta.ID,
		"findings":     "evidence landed on the intended decision",
		"verdict":      "accepted",
	})
	if err != nil {
		t.Fatalf("measure with artifact_ref: %v", err)
	}
	if !strings.Contains(out, older.Meta.ID) {
		t.Fatalf("measure response %q does not reference intended target %q", out, older.Meta.ID)
	}
	for _, want := range []string{"not approval", "not gate passage", "not claim truth", "not global truth", "not publication", "EvidencePath"} {
		if !strings.Contains(out, want) {
			t.Fatalf("measure response missing authority boundary %q:\n%s", want, out)
		}
	}
}

func TestHandleQuintDecision_MeasureRejectsMalformedMeasurements(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Keep measurement parsing strict",
		ProblemStatement: "Serve-mode measurement must reject malformed arrays without truncation.",
		WhySelected:      "Serve-mode measure should reject malformed arrays instead of truncating them.",
		SelectionPolicy:  "Prefer payload validation that preserves semantic parity with the direct tool path.",
		CounterArgument:  "Strict parsing can reject callers that relied on historical truncation.",
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
