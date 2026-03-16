package artifact

import (
	"context"
	"strings"
	"testing"
)

func TestDecide_FullDRR(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// Set up problem and portfolio
	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Event infrastructure", Signal: "DB polling at 70% CPU", Context: "events",
	})
	portfolio, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Kafka", WeakestLink: "ops complexity"},
			{Title: "NATS JetStream", WeakestLink: "ecosystem maturity"},
		},
	})

	input := DecideInput{
		ProblemRef:    prob.Meta.ID,
		PortfolioRef:  portfolio.Meta.ID,
		SelectedTitle: "NATS JetStream",
		WhySelected:   "2x throughput headroom, minimal ops for 4-person team",
		WhyNotOthers: []RejectionReason{
			{Variant: "Kafka", Reason: "Ops burden disproportionate at current scale"},
		},
		Invariants:    []string{"Every event delivered at-least-once", "Ordering preserved per stream"},
		PreConditions: []string{"NATS cluster provisioned in staging", "Load test harness ready"},
		PostConditions: []string{
			"All 12 producers migrated",
			"Load test passed: 100k events/sec, p99 < 50ms",
			"DB polling path alive as hot standby for 30 days",
		},
		Admissibility:  []string{"Fire-and-forget delivery", "Single-node production deployment"},
		EvidenceReqs:   []string{"Load test at 100k events/sec", "Producer error rate < 0.1%"},
		RefreshTriggers: []string{"Throughput exceeds 80k/s sustained", "NATS major CVE"},
		WeakestLink:    "Ecosystem maturity — fewer case studies at >50k events/sec",
		ValidUntil:     "2026-09-16T00:00:00Z",
		Rollback: &RollbackSpec{
			Triggers:    []string{"Producer error rate > 1% for > 5 minutes"},
			Steps:       []string{"Feature flag: route events back to DB polling", "Drain NATS queues"},
			BlastRadius: "All 12 services see temporary dual-delivery",
		},
		AffectedFiles: []string{"internal/events/producer.go", "internal/events/consumer.go"},
	}

	a, filePath, err := Decide(ctx, store, quintDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Kind != KindDecisionRecord {
		t.Errorf("kind = %q", a.Meta.Kind)
	}
	if a.Meta.Context != "events" {
		t.Errorf("context = %q, want events (inherited)", a.Meta.Context)
	}
	if filePath == "" {
		t.Error("file path should not be empty")
	}

	// Check all sections present
	requiredSections := []string{
		"## Selected Variant",
		"## Why Selected",
		"## Why Not Others",
		"## Invariants",
		"## Pre-conditions",
		"## Post-conditions",
		"## Admissibility",
		"## Evidence Requirements",
		"## Rollback Plan",
		"## Refresh Triggers",
		"## Weakest Link",
	}
	for _, section := range requiredSections {
		if !strings.Contains(a.Body, section) {
			t.Errorf("missing section: %s", section)
		}
	}

	// Check content
	if !strings.Contains(a.Body, "NATS JetStream") {
		t.Error("missing selected variant name")
	}
	if !strings.Contains(a.Body, "at-least-once") {
		t.Error("missing invariant content")
	}
	if !strings.Contains(a.Body, "NOT: Fire-and-forget") {
		t.Error("missing admissibility content")
	}
	if !strings.Contains(a.Body, "- [ ] All 12 producers migrated") {
		t.Error("missing post-condition checklist")
	}

	// Check links
	links, _ := store.GetLinks(ctx, a.Meta.ID)
	if len(links) != 2 {
		t.Errorf("expected 2 links (problem + portfolio), got %d", len(links))
	}

	// Check affected files
	files, _ := store.GetAffectedFiles(ctx, a.Meta.ID)
	if len(files) != 2 {
		t.Errorf("expected 2 affected files, got %d", len(files))
	}
}

func TestDecide_Tactical(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle:   "x/time/rate for rate limiting",
		WhySelected:     "Zero deps, per-IP tracking testable in Go",
		Invariants:      []string{"Rate limit applied per-IP"},
		RefreshTriggers: []string{"Traffic > 10x current"},
		Mode:            "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Mode != ModeTactical {
		t.Errorf("mode = %q, want tactical", a.Meta.Mode)
	}
	if !strings.Contains(a.Body, "## Invariants") {
		t.Error("tactical mode should still have invariants")
	}
}

func TestDecide_MissingRequired(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Missing selected_title
	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		WhySelected: "because",
	})
	if err == nil {
		t.Error("expected error for missing selected_title")
	}

	// Missing why_selected
	_, _, err = Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "something",
	})
	if err == nil {
		t.Error("expected error for missing why_selected")
	}
}

func TestApply_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	dec, _, _ := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle:  "NATS JetStream",
		WhySelected:    "Ops simplicity",
		Invariants:     []string{"At-least-once delivery"},
		PreConditions:  []string{"Staging cluster ready"},
		PostConditions: []string{"All producers migrated"},
		Admissibility:  []string{"Fire-and-forget"},
		EvidenceReqs:   []string{"Load test at 100k/s"},
		Rollback: &RollbackSpec{
			Triggers: []string{"Error rate > 1%"},
			Steps:    []string{"Switch back to DB polling"},
		},
	})

	brief, err := Apply(ctx, store, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Check brief structure
	requiredBriefSections := []string{
		"## What to Implement",
		"## Invariants (MUST hold)",
		"## NOT Acceptable",
		"## Before Starting",
		"## Definition of Done",
		"## Evidence to Collect",
		"## If Things Go Wrong",
	}
	for _, section := range requiredBriefSections {
		if !strings.Contains(brief, section) {
			t.Errorf("brief missing section: %s", section)
		}
	}

	if !strings.Contains(brief, "At-least-once delivery") {
		t.Error("brief missing invariant content")
	}
	if !strings.Contains(brief, dec.Meta.ID) {
		t.Error("brief missing decision reference")
	}
}

func TestApply_NotFound(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := Apply(ctx, store, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent decision")
	}
}

func TestDecide_InheritsContext(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Auth redesign", Signal: "Token expiry issues", Context: "auth",
	})

	a, _, err := Decide(ctx, store, quintDir, DecideInput{
		ProblemRef:    prob.Meta.ID,
		SelectedTitle: "JWT with refresh tokens",
		WhySelected:   "Standard approach, well-understood",
	})
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Context != "auth" {
		t.Errorf("context = %q, want auth (inherited from problem)", a.Meta.Context)
	}
}
