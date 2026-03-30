package artifact

import (
	"context"
	"strings"
	"testing"
)

func TestDecide_FullDRR(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Set up problem and portfolio
	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Event infrastructure", Signal: "DB polling at 70% CPU", Context: "events",
		Constraints: []string{"Must maintain <500ms p99"},
		Acceptance:  "All producers on new infra, p99 < 50ms",
	})
	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
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

	a, filePath, err := Decide(ctx, store, haftDir, input)
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

	// Check FPF E.9 four-component structure
	requiredSections := []string{
		"## 1. Problem Frame",
		"## 2. Decision",
		"## 3. Rationale",
		"## 4. Consequences",
	}
	for _, section := range requiredSections {
		if !strings.Contains(a.Body, section) {
			t.Errorf("missing FPF E.9 component: %s", section)
		}
	}

	// Check Problem Frame pulled from ProblemCard
	if !strings.Contains(a.Body, "DB polling at 70% CPU") {
		t.Error("Problem Frame should contain signal from ProblemCard")
	}
	if !strings.Contains(a.Body, "500ms p99") {
		t.Error("Problem Frame should contain constraints from ProblemCard")
	}

	// Check Decision contract
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

	// Check Rationale
	if !strings.Contains(a.Body, "Kafka") && !strings.Contains(a.Body, "Rejected") {
		t.Error("missing rejection rationale")
	}
	if !strings.Contains(a.Body, "Ecosystem maturity") {
		t.Error("missing weakest link")
	}

	// Check Consequences
	if !strings.Contains(a.Body, "Rollback") {
		t.Error("missing rollback plan")
	}
	if !strings.Contains(a.Body, "Refresh triggers") {
		t.Error("missing refresh triggers")
	}
	if !strings.Contains(a.Body, "producer.go") {
		t.Error("missing affected files")
	}

	// Check links
	links, _ := store.GetLinks(ctx, a.Meta.ID)
	if len(links) != 2 {
		t.Errorf("expected 2 links (problem + portfolio), got %d", len(links))
	}

	// Check affected files in DB
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
	if !strings.Contains(a.Body, "Rate limit applied per-IP") {
		t.Error("tactical mode should still have invariants")
	}
	// Tactical without problem_ref: Problem Frame section exists but minimal
	if !strings.Contains(a.Body, "## 1. Problem Frame") {
		t.Error("even tactical DRR should have Problem Frame section")
	}
}

func TestDecide_MissingRequired(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		WhySelected: "because",
	})
	if err == nil {
		t.Error("expected error for missing selected_title")
	}

	_, _, err = Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "something",
	})
	if err == nil {
		t.Error("expected error for missing why_selected")
	}
}

func TestApply_ReturnsBody(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	dec, _, _ := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "NATS JetStream",
		WhySelected:   "Ops simplicity",
		Invariants:    []string{"At-least-once delivery"},
	})

	body, err := Apply(ctx, store, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Apply now returns the DRR body directly
	if !strings.Contains(body, "NATS JetStream") {
		t.Error("apply should return DRR body with decision content")
	}
	if !strings.Contains(body, "At-least-once delivery") {
		t.Error("apply should return DRR body with invariants")
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
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Auth redesign", Signal: "Token expiry issues", Context: "auth",
	})

	a, _, err := Decide(ctx, store, haftDir, DecideInput{
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
