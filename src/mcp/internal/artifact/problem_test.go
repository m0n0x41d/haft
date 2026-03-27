package artifact

import (
	"context"
	"strings"
	"testing"
)

func TestFrameProblem_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	input := ProblemFrameInput{
		Title:   "Webhook delivery unreliable",
		Signal:  "Payment webhook retries hitting 15% failure rate",
		Context: "payments",
		Constraints: []string{
			"Must maintain <500ms p99 latency",
			"Cannot break existing merchant integrations",
		},
		OptimizationTargets:  []string{"Reduce webhook failure rate to <1%"},
		ObservationIndicators: []string{"CPU utilization on webhook workers"},
		Acceptance:           "Failure rate below 1% for 7 consecutive days",
		BlastRadius:          "All merchant integrations",
		Reversibility:        "medium",
	}

	a, filePath, err := FrameProblem(ctx, store, quintDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Kind != KindProblemCard {
		t.Errorf("kind = %q", a.Meta.Kind)
	}
	if a.Meta.Title != "Webhook delivery unreliable" {
		t.Errorf("title = %q", a.Meta.Title)
	}
	if a.Meta.Context != "payments" {
		t.Errorf("context = %q", a.Meta.Context)
	}
	if a.Meta.Mode != ModeStandard {
		t.Errorf("mode = %q, want standard", a.Meta.Mode)
	}
	if filePath == "" {
		t.Error("file path should not be empty")
	}

	// Check body contains all sections
	if !strings.Contains(a.Body, "## Signal") {
		t.Error("missing Signal section")
	}
	if !strings.Contains(a.Body, "## Constraints") {
		t.Error("missing Constraints section")
	}
	if !strings.Contains(a.Body, "## Optimization Targets") {
		t.Error("missing Optimization Targets section")
	}
	if !strings.Contains(a.Body, "## Observation Indicators") {
		t.Error("missing Observation Indicators section")
	}
	if !strings.Contains(a.Body, "## Acceptance") {
		t.Error("missing Acceptance section")
	}
	if !strings.Contains(a.Body, "## Blast Radius") {
		t.Error("missing Blast Radius section")
	}

	// Verify searchable
	results, err := store.Search(ctx, "webhook failure rate", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("problem not found via search")
	}
}

func TestFrameProblem_MissingTitle(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := FrameProblem(ctx, store, t.TempDir(), ProblemFrameInput{
		Signal: "something is broken",
	})
	if err == nil {
		t.Error("expected error for missing title")
	}
}

func TestFrameProblem_MissingSignal(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := FrameProblem(ctx, store, t.TempDir(), ProblemFrameInput{
		Title: "Some problem",
	})
	if err == nil {
		t.Error("expected error for missing signal")
	}
}

func TestFrameProblem_TacticalMode(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a, _, err := FrameProblem(ctx, store, t.TempDir(), ProblemFrameInput{
		Title:  "Pick a rate limiter",
		Signal: "Need rate limiting on public API",
		Mode:   "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.Meta.Mode != ModeTactical {
		t.Errorf("mode = %q, want tactical", a.Meta.Mode)
	}
}

func TestCharacterizeProblem_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// Create a problem first
	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title:  "Event infrastructure",
		Signal: "DB polling hitting 70% CPU",
	})

	// Characterize it
	input := CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "throughput", ScaleType: "ratio", Unit: "events/sec", Polarity: "higher_better", HowToMeasure: "Load test at sustained rate"},
			{Name: "ops complexity", ScaleType: "ordinal", Polarity: "lower_better", HowToMeasure: "On-call pages per month"},
			{Name: "cost", ScaleType: "ratio", Unit: "USD/month", Polarity: "lower_better"},
		},
		ParityRules: "All candidates tested with same 50k events/sec load profile",
	}

	a, _, err := CharacterizeProblem(ctx, store, quintDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "## Characterization v1") {
		t.Error("missing Characterization v1 section")
	}
	if !strings.Contains(a.Body, "throughput") {
		t.Error("missing throughput dimension")
	}
	if !strings.Contains(a.Body, "Parity rules:") {
		t.Error("missing parity rules")
	}
	if a.Meta.Version != 2 {
		t.Errorf("version = %d, want 2 after update", a.Meta.Version)
	}
}

func TestCharacterizeProblem_MissingRef(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := CharacterizeProblem(ctx, store, t.TempDir(), CharacterizeInput{
		Dimensions: []ComparisonDimension{{Name: "speed"}},
	})
	if err == nil {
		t.Error("expected error for missing problem_ref")
	}
}

func TestCharacterizeProblem_NoDimensions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	prob, _, _ := FrameProblem(ctx, store, t.TempDir(), ProblemFrameInput{
		Title: "Test", Signal: "test signal",
	})

	_, _, err := CharacterizeProblem(ctx, store, t.TempDir(), CharacterizeInput{
		ProblemRef: prob.Meta.ID,
	})
	if err == nil {
		t.Error("expected error for no dimensions")
	}
}

func TestCharacterizeProblem_VersionsNotOverwrites(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test", Signal: "test",
	})

	// First characterization
	CharacterizeProblem(ctx, store, quintDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{{Name: "speed"}},
	})

	// Second characterization appends, not replaces
	a, _, err := CharacterizeProblem(ctx, store, quintDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{{Name: "reliability"}, {Name: "cost"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should have BOTH versions
	if !strings.Contains(a.Body, "## Characterization v1") {
		t.Error("missing v1 characterization")
	}
	if !strings.Contains(a.Body, "## Characterization v2") {
		t.Error("missing v2 characterization")
	}
	if !strings.Contains(a.Body, "speed") {
		t.Error("v1 dimension 'speed' should be preserved in history")
	}
	if !strings.Contains(a.Body, "reliability") {
		t.Error("missing new dimension 'reliability'")
	}
	if !strings.Contains(a.Body, "cost") {
		t.Error("missing new dimension 'cost'")
	}
}

func TestSelectProblems(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Problem A", Signal: "signal A", Context: "auth",
	})
	FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Problem B", Signal: "signal B", Context: "payments",
	})
	FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Problem C", Signal: "signal C", Context: "auth",
	})

	// All problems
	all, err := SelectProblems(ctx, store, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 problems, got %d", len(all))
	}

	// Filter by context
	auth, err := SelectProblems(ctx, store, "auth", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(auth) != 2 {
		t.Errorf("expected 2 auth problems, got %d", len(auth))
	}
}

func TestFindActiveProblem(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// No problems yet
	p, err := FindActiveProblem(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Error("expected nil for no problems")
	}

	// Create one
	FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test Problem", Signal: "test signal", Context: "test",
	})

	p, err = FindActiveProblem(ctx, store, "test")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected to find active problem")
	}
	if p.Meta.Title != "Test Problem" {
		t.Errorf("title = %q", p.Meta.Title)
	}
}

func TestSelectProblems_ExcludesDeprecated(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	a, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Active Problem", Signal: "signal",
	})
	b, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Deprecated Problem", Signal: "signal",
	})

	DeprecateArtifact(ctx, store, quintDir, b.Meta.ID, "no longer relevant")

	// Without context filter
	problems, err := SelectProblems(ctx, store, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("expected 1 active problem, got %d", len(problems))
	}
	if problems[0].Meta.ID != a.Meta.ID {
		t.Errorf("expected %s, got %s", a.Meta.ID, problems[0].Meta.ID)
	}

	// With context filter — same context, same expectation
	c, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Active in ctx", Signal: "signal", Context: "payments",
	})
	d, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Deprecated in ctx", Signal: "signal", Context: "payments",
	})
	DeprecateArtifact(ctx, store, quintDir, d.Meta.ID, "done")

	ctxProblems, err := SelectProblems(ctx, store, "payments", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxProblems) != 1 {
		t.Fatalf("expected 1 active problem in context, got %d", len(ctxProblems))
	}
	if ctxProblems[0].Meta.ID != c.Meta.ID {
		t.Errorf("expected %s, got %s", c.Meta.ID, ctxProblems[0].Meta.ID)
	}
}

func TestFindActiveProblem_ExcludesDeprecated(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Only Problem", Signal: "signal",
	})

	// Verify it's found
	p, err := FindActiveProblem(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected to find active problem")
	}

	// Deprecate it
	DeprecateArtifact(ctx, store, quintDir, p.Meta.ID, "done")

	// Should no longer be found
	p, err = FindActiveProblem(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Errorf("expected nil after deprecation, got %s", p.Meta.ID)
	}
}

// FormatProblemResponse tests moved to internal/present/format_test.go
