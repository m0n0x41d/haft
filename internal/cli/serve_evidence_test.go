package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/reff"
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
	result, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
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
	for _, want := range []string{"not approval", "not gate passage", "not global truth", "EvidencePath"} {
		if !strings.Contains(result, want) {
			t.Fatalf("evidence response missing authority boundary %q:\n%s", want, result)
		}
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

func TestHandleQuintQuery_EvidencePathBlocksLegacyFormalityWhenCurrentRequired(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Require current-formality evidence",
		WhySelected:     "Need a decision artifact with claim-scoped legacy evidence.",
		SelectionPolicy: "Use the smallest decision that exercises stronger evidence-path reliance.",
		CounterArgument: "A synthetic decision only validates the projection contract.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Use unbound evidence",
			Reason:  "The test needs claim binding so only formality can block bounded reliance.",
		}},
		WeakestLink: "legacy formality bridge",
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"EvidencePath stops blocking legacy formality for current-formality attempted use"},
		},
		Predictions: []artifact.PredictionInput{{
			Claim:      "Release evidence must be current-formality evidence",
			Observable: "formality scale",
			Threshold:  "current F0-F9",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Get(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	claims := reloaded.UnmarshalDecisionFields().Claims
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %+v", claims)
	}

	validUntil := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	item, err := artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef:      decision.Meta.ID,
		Content:          "Legacy checker supports the claim but uses old formality semantics.",
		Type:             "test",
		Verdict:          "supports",
		CarrierRef:       "internal/cli/serve_evidence_test.go",
		CongruenceLevel:  3,
		FormalityLevel:   2,
		FormalityScaleID: reff.FormalityScaleLegacy,
		ClaimRefs:        []string{claims[0].ID},
		ValidUntil:       validUntil,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, haftDir, map[string]any{
		"action":                     "evidence_path",
		"artifact_ref":               decision.Meta.ID,
		"evidence_ref":               item.ID,
		"claim_ref":                  claims[0].ID,
		"attempted_use":              "release reliance for the declared claim",
		"requires_current_formality": true,
		"method_ref":                 "mpull-test-evidence-path",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery evidence_path returned error: %v", err)
	}

	var record artifact.EvidencePathRecord
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode evidence_path record: %v\n%s", err, result)
	}

	if record.AttemptedUse.RequiresCurrentFormality != true {
		t.Fatalf("attempted_use = %+v, want current formality requirement", record.AttemptedUse)
	}
	if record.RelianceDisposition.Disposition != artifact.EvidenceRelianceBlocked {
		t.Fatalf("reliance = %+v, want blocked", record.RelianceDisposition)
	}
	if record.RelianceDisposition.Reason != "current_formality_required" {
		t.Fatalf("reason = %q, want current_formality_required", record.RelianceDisposition.Reason)
	}
	if record.AuthorityBoundary.Approval != artifact.EvidenceBoundaryNotApproval {
		t.Fatalf("approval boundary = %q", record.AuthorityBoundary.Approval)
	}
	if record.AuthorityBoundary.GateDecision != artifact.EvidenceBoundaryNotGateDecision {
		t.Fatalf("gate boundary = %q", record.AuthorityBoundary.GateDecision)
	}
	if record.AuthorityBoundary.GlobalTruth != artifact.EvidenceBoundaryNotGlobalTruth {
		t.Fatalf("truth boundary = %q", record.AuthorityBoundary.GlobalTruth)
	}
}

func TestHandleQuintDecision_EvidencePersistsClaimBinding(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Attach claim-scoped evidence",
		WhySelected:     "Need a decision artifact for validating claim_refs and claim_scope on the serve path.",
		SelectionPolicy: "Prefer the smallest realistic decision record that still exercises claim binding.",
		CounterArgument: "A synthetic decision can miss the coupling of a compare-driven decision lifecycle.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Attach evidence to a note",
			Reason:  "This serve-path test needs decision-scoped claim metadata.",
		}},
		WeakestLink: "The decision is synthetic and therefore weaker than a real compared choice.",
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Evidence attachment stops preserving claim bindings"},
		},
		Predictions: []artifact.PredictionInput{{
			Claim:      "Throughput stays above 100k events/sec",
			Observable: "throughput",
			Threshold:  "> 100k events/sec",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Get(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	claims := reloaded.UnmarshalDecisionFields().Claims
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %+v", claims)
	}

	_, _, err = handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "evidence",
		"artifact_ref":     decision.Meta.ID,
		"evidence_content": "Replay benchmark supports the throughput expectation.",
		"evidence_type":    "benchmark",
		"evidence_verdict": "supports",
		"claim_refs":       []string{claims[0].ID},
		"claim_scope":      []string{"throughput", "warmup"},
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := store.GetEvidenceItems(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(items))
	}

	if strings.Join(items[0].ClaimRefs, ",") != claims[0].ID {
		t.Fatalf("claim_refs = %v", items[0].ClaimRefs)
	}
	if strings.Join(items[0].ClaimScope, ",") != "throughput,warmup" {
		t.Fatalf("claim_scope = %v", items[0].ClaimScope)
	}
}

func TestHandleQuintQuery_EvidencePathBuildsBoundedReliance(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Bind evidence reliance to one claim",
		WhySelected:     "Need a decision artifact with an explicit claim for EvidencePath projection.",
		SelectionPolicy: "Prefer the smallest realistic decision record that still carries an explicit claim.",
		CounterArgument: "A synthetic decision can miss lifecycle coupling from a real compared choice.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Use a raw evidence row",
			Reason:  "EvidencePath must prove claim binding against an artifact-scoped evidence item.",
		}},
		WeakestLink: "The decision is synthetic and therefore only validates the projection contract.",
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"EvidencePath stops preserving authority boundaries"},
		},
		Predictions: []artifact.PredictionInput{{
			Claim:      "The verifier remains deterministic",
			Observable: "verifier result",
			Threshold:  "same input yields same result",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Get(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	claims := reloaded.UnmarshalDecisionFields().Claims
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %+v", claims)
	}

	validUntil := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	item, err := artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef:     decision.Meta.ID,
		Content:         "Verifier replay produced the same result for the same input.",
		Type:            "test",
		Verdict:         "supports",
		CarrierRef:      "internal/cli/serve_evidence_test.go",
		CongruenceLevel: 3,
		FormalityLevel:  7,
		ClaimRefs:       []string{claims[0].ID},
		ValidUntil:      validUntil,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, haftDir, map[string]any{
		"action":        "evidence_path",
		"artifact_ref":  decision.Meta.ID,
		"evidence_ref":  item.ID,
		"claim_ref":     claims[0].ID,
		"attempted_use": "verification reliance for the declared deterministic claim",
		"method_ref":    "mpull-test-evidence-path",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery evidence_path returned error: %v", err)
	}

	var record artifact.EvidencePathRecord
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode evidence_path record: %v\n%s", err, result)
	}

	if record.RelianceDisposition.Disposition != artifact.EvidenceRelianceBounded {
		t.Fatalf("reliance = %+v, want bounded", record.RelianceDisposition)
	}
	if record.ClaimBinding.Status != artifact.EvidenceClaimBindingBound {
		t.Fatalf("claim_binding = %+v, want bound", record.ClaimBinding)
	}
	if record.TraceBinding.Status != artifact.EvidenceTraceBindingDeclared {
		t.Fatalf("trace_binding = %+v, want declared", record.TraceBinding)
	}
	if record.AuthorityBoundary.Approval != artifact.EvidenceBoundaryNotApproval {
		t.Fatalf("approval boundary = %q", record.AuthorityBoundary.Approval)
	}
	if record.AuthorityBoundary.GateDecision != artifact.EvidenceBoundaryNotGateDecision {
		t.Fatalf("gate boundary = %q", record.AuthorityBoundary.GateDecision)
	}
	if record.AuthorityBoundary.GlobalTruth != artifact.EvidenceBoundaryNotGlobalTruth {
		t.Fatalf("truth boundary = %q", record.AuthorityBoundary.GlobalTruth)
	}
}
