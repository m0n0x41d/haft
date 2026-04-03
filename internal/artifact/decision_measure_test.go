package artifact

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

func TestMeasure_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "NATS JetStream",
		WhySelected:   "Ops simplicity",
		ValidUntil:    "2027-01-01T00:00:00Z",
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

	a, err := Measure(ctx, store, haftDir, input)
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
	if items[0].FormalityLevel != 2 {
		t.Errorf("evidence formality = %d, want 2", items[0].FormalityLevel)
	}
	if items[0].ValidUntil != "2027-01-01T00:00:00Z" {
		t.Errorf("evidence valid_until = %q, want propagated decision validity", items[0].ValidUntil)
	}
	if len(items[0].ClaimScope) != 1 || items[0].ClaimScope[0] != "Load test at 100k/s passed (actual: 120k/s)" {
		t.Errorf("evidence claim_scope = %#v, want criteria_met scope", items[0].ClaimScope)
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
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
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
		ClaimScope:      []string{"throughput", "latency", "throughput"},
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
	if items[0].FormalityLevel != 2 {
		t.Errorf("stored formality = %d, want normalized F2", items[0].FormalityLevel)
	}
	if got := strings.Join(items[0].ClaimScope, ","); got != "latency,throughput" {
		t.Errorf("stored claim_scope = %q, want deduplicated scope", got)
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
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	// Add supporting evidence (CL3 = same context)
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Load test passed",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2026-09-01T00:00:00Z",
	})

	// Add weakening evidence with lower CL (CL1 = different context)
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

func TestWLNKSummary_SurfacesAssuranceCoverage(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Queue migration",
		Signal: "Current queue saturates under burst load.",
		Acceptance: strings.Join([]string{
			"- P99 latency under 50ms",
			"- Throughput above 100k events/sec",
		}, "\n"),
	})
	if err != nil {
		t.Fatal(err)
	}

	dec, _, err := Decide(ctx, store, haftDir, DecideInput{
		ProblemRef:    prob.Meta.ID,
		SelectedTitle: "JetStream",
		WhySelected:   "Lower operational overhead",
		WeakestLink:   "benchmark evidence freshness",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Load test passed on staging",
		Type:            "benchmark",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  7,
		ClaimScope:      []string{"P99 latency under 50ms"},
	})
	if err != nil {
		t.Fatal(err)
	}

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	if wlnk.FEff != 2 {
		t.Errorf("FEff = %d, want 2", wlnk.FEff)
	}
	if !wlnk.CoverageKnown {
		t.Fatal("expected explicit acceptance coverage")
	}
	if got := strings.Join(wlnk.GEff, ","); got != "P99 latency under 50ms" {
		t.Errorf("GEff = %q, want covered criterion", got)
	}
	if got := strings.Join(wlnk.CoverageGaps, ","); got != "Throughput above 100k events/sec" {
		t.Errorf("CoverageGaps = %q, want uncovered criterion", got)
	}
	if !strings.Contains(wlnk.Summary, "Assurance: F2 (structured-formal)") {
		t.Errorf("summary should show structured assurance: %q", wlnk.Summary)
	}
	if !strings.Contains(wlnk.Summary, "G: 1/2 criteria covered") {
		t.Errorf("summary should show coverage ratio: %q", wlnk.Summary)
	}
}

func TestWLNKSummary_Refuting(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
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

// --- R_eff computation tests ---

func TestREff_NoEvidence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-reff-001", Kind: KindDecisionRecord, Title: "D"},
		Body: "d",
	})

	wlnk := ComputeWLNKSummary(ctx, store, "dec-reff-001")
	if wlnk.HasEvidence {
		t.Error("HasEvidence should be false with no evidence")
	}
	if wlnk.REff != 0.0 {
		t.Errorf("REff = %.2f, want 0.0 (default) when no evidence", wlnk.REff)
	}
	if !strings.Contains(wlnk.Summary, "no evidence attached") {
		t.Errorf("summary = %q, want 'no evidence attached'", wlnk.Summary)
	}
}

func TestREff_AllSupporting(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Test A passed",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Test B passed",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	if !wlnk.HasEvidence {
		t.Error("HasEvidence should be true")
	}
	// supports=1.0, CL3 penalty=0.0 → effective=1.0, min=1.0
	assertREff(t, wlnk.REff, 1.0)
	if !strings.Contains(wlnk.Summary, "R: 1.00") {
		t.Errorf("summary should show R_eff: %q", wlnk.Summary)
	}
}

func TestREff_MixedVerdicts(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Test passed",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Partial result",
		Verdict:         "weakens",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	// supports=1.0, weakens=0.5, both CL3 → min=0.5
	assertREff(t, wlnk.REff, 0.5)
}

func TestREff_AllRefuting(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Crashed",
		Verdict:         "refutes",
		CongruenceLevel: 3,
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	// refutes=0.0, CL3 penalty=0.0 → effective=0.0
	assertREff(t, wlnk.REff, 0.0)
}

func TestREff_CLPenalty(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	// CL2: supports(1.0) - 0.1 = 0.9
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Similar context evidence",
		Verdict:         "supports",
		CongruenceLevel: 2,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	assertREff(t, wlnk.REff, 0.9)
}

func TestREff_CL1Penalty(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	// CL1: supports(1.0) - 0.4 = 0.6
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Different context evidence",
		Verdict:         "supports",
		CongruenceLevel: 1,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	assertREff(t, wlnk.REff, 0.6)
}

func TestREff_CL0Penalty(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	// CL0: supports(1.0) - 0.9 = 0.1
	// This also verifies CL=0 is NOT silently upgraded to CL=3 (known issue S4)
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Opposed context evidence",
		Verdict:         "supports",
		CongruenceLevel: 0,
		FormalityLevel:  0, // also 0 — should NOT be defaulted
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	assertREff(t, wlnk.REff, 0.1)
}

func TestREff_ExpiredEvidence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	// One fresh supporting, one expired supporting
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Fresh test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "Old benchmark",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2020-01-01T00:00:00Z", // expired
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	// Fresh: 1.0, Expired: 0.1 → min = 0.1
	assertREff(t, wlnk.REff, 0.1)
	if !strings.Contains(wlnk.Summary, "STALE") {
		t.Errorf("summary should mention STALE: %q", wlnk.Summary)
	}
}

func TestREff_WeakensWithCLPenalty(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	dec, _, _ := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Test",
		WhySelected:   "Because",
	})

	// weakens(0.5) with CL1 penalty(0.4) = 0.1
	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "External partial result",
		Verdict:         "weakens",
		CongruenceLevel: 1,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	wlnk := ComputeWLNKSummary(ctx, store, dec.Meta.ID)
	assertREff(t, wlnk.REff, 0.1)
}

// --- Scoring pure function tests ---

func TestVerdictToScore(t *testing.T) {
	cases := []struct {
		verdict string
		want    float64
	}{
		{"supports", 1.0},
		{"accepted", 1.0},
		{"weakens", 0.5},
		{"partial", 0.5},
		{"refutes", 0.0},
		{"failed", 0.0},
		{"unknown", 0.5},
		{"", 0.5},
	}
	for _, tc := range cases {
		got := reff.VerdictToScore(tc.verdict)
		if got != tc.want {
			t.Errorf("VerdictToScore(%q) = %.1f, want %.1f", tc.verdict, got, tc.want)
		}
	}
}

func TestCLPenalty(t *testing.T) {
	cases := []struct {
		cl   int
		want float64
	}{
		{3, 0.0},
		{2, 0.1},
		{1, 0.4},
		{0, 0.9},
		{-1, 0.9}, // invalid treated as CL0
	}
	for _, tc := range cases {
		got := reff.CLPenalty(tc.cl)
		if got != tc.want {
			t.Errorf("CLPenalty(%d) = %.1f, want %.1f", tc.cl, got, tc.want)
		}
	}
}

func TestScoreEvidence_Decay(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// Expired evidence always scores 0.1
	expired := EvidenceItem{
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2026-01-01T00:00:00Z",
	}
	got := scoreEvidence(expired, now)
	assertREff(t, got, 0.1)

	// Fresh evidence scored normally
	fresh := EvidenceItem{
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	}
	got = scoreEvidence(fresh, now)
	assertREff(t, got, 1.0)

	// No valid_until = perpetual, scored normally
	perpetual := EvidenceItem{
		Verdict:         "supports",
		CongruenceLevel: 2,
	}
	got = scoreEvidence(perpetual, now)
	assertREff(t, got, 0.9) // 1.0 - 0.1 CL2 penalty
}

func assertREff(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("R_eff = %.4f, want %.4f", got, want)
	}
}
