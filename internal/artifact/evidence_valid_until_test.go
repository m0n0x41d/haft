package artifact

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAttachEvidence_ExpiredValidUntilDegradesREffAndScan(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Fresh decision",
		WhySelected:   "Keep the decision itself active while attached evidence expires",
		ValidUntil:    time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}))
	if err != nil {
		t.Fatal(err)
	}

	expiredEvidence := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	_, err = AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     decision.Meta.ID,
		Content:         "Staging benchmark from last quarter",
		Type:            "benchmark",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      expiredEvidence,
	})
	if err != nil {
		t.Fatal(err)
	}

	wlnk := ComputeWLNKSummary(ctx, store, decision.Meta.ID)
	if wlnk.REff != 0.1 {
		t.Fatalf("REff = %.2f, want 0.10 for expired attached evidence", wlnk.REff)
	}
	if wlnk.MinFreshness != expiredEvidence {
		t.Fatalf("MinFreshness = %q, want %q", wlnk.MinFreshness, expiredEvidence)
	}
	if !strings.Contains(wlnk.Summary, "STALE evidence") {
		t.Fatalf("summary should mention stale evidence, got %q", wlnk.Summary)
	}

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		if item.ID != decision.Meta.ID {
			continue
		}
		if item.Category != StaleCategoryREffDegraded {
			t.Fatalf("category = %q, want %q", item.Category, StaleCategoryREffDegraded)
		}
		if !strings.Contains(item.Reason, "evidence degraded") {
			t.Fatalf("reason should mention degraded evidence, got %q", item.Reason)
		}
		if !strings.Contains(item.Reason, "R_eff: 0.10") {
			t.Fatalf("reason should expose degraded R_eff, got %q", item.Reason)
		}
		return
	}

	t.Fatal("decision with expired attached evidence should appear in stale scan")
}

func TestComputeWLNKSummary_MinFreshnessUsesParsedTimeOrdering(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Normalize freshness comparison",
		WhySelected:   "Mixed valid_until carriers should compare by time, not lexicographically",
		ValidUntil:    time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}))
	if err != nil {
		t.Fatal(err)
	}

	validUntilValues := []string{
		"2026-01-02",
		"2026-01-02T00:30:00+02:00",
		"2026-01-01T23:00:00Z",
	}

	for _, validUntil := range validUntilValues {
		_, err = AttachEvidence(ctx, store, EvidenceInput{
			ArtifactRef:     decision.Meta.ID,
			Content:         "freshness ordering fixture",
			Type:            "benchmark",
			Verdict:         "supports",
			CongruenceLevel: 3,
			ValidUntil:      validUntil,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	wlnk := ComputeWLNKSummary(ctx, store, decision.Meta.ID)
	want := "2026-01-02T00:30:00+02:00"
	if wlnk.MinFreshness != want {
		t.Fatalf("MinFreshness = %q, want %q", wlnk.MinFreshness, want)
	}
}
