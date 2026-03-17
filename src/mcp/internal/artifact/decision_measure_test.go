package artifact

import (
	"context"
	"strings"
	"testing"
)

func TestMeasure_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, quintDir, DecideInput{
		SelectedTitle: "NATS JetStream",
		WhySelected:   "Ops simplicity",
		PostConditions: []string{
			"All producers migrated",
			"Load test at 100k/s passed",
		},
	})

	input := MeasureInput{
		DecisionRef:    dec.Meta.ID,
		Findings:       "Migration completed. 11/12 producers live. Load test passed at 120k/s.",
		CriteriaMet:    []string{"Load test at 100k/s passed (actual: 120k/s)"},
		CriteriaNotMet: []string{"All producers migrated (11/12, payments-legacy pending)"},
		Measurements:   []string{"p99 latency: 42ms", "throughput: 120k events/sec"},
		Verdict:        "partial",
	}

	a, err := Measure(ctx, store, quintDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "## Impact Measurement") {
		t.Error("missing Impact Measurement section")
	}
	if !strings.Contains(a.Body, "partial") {
		t.Error("missing verdict")
	}
	if !strings.Contains(a.Body, "120k events/sec") {
		t.Error("missing measurement")
	}
	if !strings.Contains(a.Body, "[x]") {
		t.Error("missing criteria met checklist")
	}
	if !strings.Contains(a.Body, "[ ]") {
		t.Error("missing criteria not met checklist")
	}

	// Evidence item should be recorded
	items, _ := store.GetEvidenceItems(ctx, dec.Meta.ID)
	if len(items) == 0 {
		t.Error("expected evidence item from measurement")
	}
	if items[0].Verdict != "partial" {
		t.Errorf("evidence verdict = %q, want partial", items[0].Verdict)
	}
}

func TestMeasure_MissingRequired(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := Measure(ctx, store, t.TempDir(), MeasureInput{DecisionRef: "x"})
	if err == nil {
		t.Error("expected error for missing findings")
	}

	_, err = Measure(ctx, store, t.TempDir(), MeasureInput{DecisionRef: "x", Findings: "y"})
	if err == nil {
		t.Error("expected error for missing verdict")
	}
}

func TestAttachEvidence_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, quintDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	item, err := AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Load test: 100k events/sec, p99 < 50ms",
		Type:            "benchmark",
		Verdict:         "supports",
		CarrierRef:      "benchmarks/nats_load_test.md",
		CongruenceLevel: 3,
		FormalityLevel:  7,
		ValidUntil:      "2026-06-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	if item.ID == "" {
		t.Error("evidence ID should not be empty")
	}
	if item.Type != "benchmark" {
		t.Errorf("type = %q", item.Type)
	}

	// Verify stored
	items, _ := store.GetEvidenceItems(ctx, dec.Meta.ID)
	if len(items) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(items))
	}
}

func TestAttachEvidence_MissingArtifact(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef: "nonexistent",
		Content:     "test",
	})
	if err == nil {
		t.Error("expected error for nonexistent artifact")
	}
}

func TestWLNKSummary_NoEvidence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "D"},
		Body: "d",
	})

	wlnk := ComputeWLNKSummary(ctx, store, "dec-001")
	if wlnk.EvidenceCount != 0 {
		t.Errorf("expected 0 evidence, got %d", wlnk.EvidenceCount)
	}
	if wlnk.Summary != "no evidence attached" {
		t.Errorf("summary = %q", wlnk.Summary)
	}
}

func TestWLNKSummary_WithEvidence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, quintDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	// Add supporting evidence
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef: dec.Meta.ID,
		Content:     "Load test passed",
		Verdict:     "supports",
		ValidUntil:  "2026-09-01T00:00:00Z",
	})

	// Add weakening evidence with lower CL
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "External benchmark shows different results",
		Verdict:         "weakens",
		CongruenceLevel: 1,
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)

	if wlnk.EvidenceCount != 2 {
		t.Errorf("evidence count = %d, want 2", wlnk.EvidenceCount)
	}
	if wlnk.Supporting != 1 {
		t.Errorf("supporting = %d, want 1", wlnk.Supporting)
	}
	if wlnk.Weakening != 1 {
		t.Errorf("weakening = %d, want 1", wlnk.Weakening)
	}
	if wlnk.WeakestCL != 1 {
		t.Errorf("weakest CL = %d, want 1 (different context)", wlnk.WeakestCL)
	}
	if !strings.Contains(wlnk.Summary, "1 weakening") {
		t.Errorf("summary should mention weakening: %q", wlnk.Summary)
	}
	if !strings.Contains(wlnk.Summary, "different context") {
		t.Errorf("summary should mention weakest CL: %q", wlnk.Summary)
	}
}

func TestWLNKSummary_Refuting(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, quintDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef: dec.Meta.ID,
		Content:     "System crashed under load",
		Verdict:     "refutes",
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	if wlnk.Refuting != 1 {
		t.Errorf("refuting = %d, want 1", wlnk.Refuting)
	}
	if !strings.Contains(wlnk.Summary, "REFUTING") {
		t.Errorf("summary should highlight REFUTING: %q", wlnk.Summary)
	}
}
