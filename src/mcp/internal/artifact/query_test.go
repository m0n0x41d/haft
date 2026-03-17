package artifact

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestQuerySearch_FindsResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "NATS JetStream for events"},
		Body: "Selected NATS over Kafka for domain event infrastructure",
	})

	result, err := QuerySearch(ctx, store, "NATS events", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "NATS JetStream") {
		t.Error("expected NATS in results")
	}
}

func TestQuerySearch_NoResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	result, err := QuerySearch(ctx, store, "nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No results") {
		t.Error("expected 'No results' message")
	}
}

func TestQuerySearch_EmptyQuery(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := QuerySearch(ctx, store, "", 10)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestQueryStatus_Dashboard(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// Create some artifacts
	FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Rate limiting", Signal: "Scraper traffic", Context: "api",
	})
	Decide(ctx, store, quintDir, DecideInput{
		SelectedTitle: "x/time/rate",
		WhySelected:   "Zero deps",
		ValidUntil:    time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339),
	})
	CreateNote(ctx, store, quintDir, NoteInput{
		Title:     "Using RWMutex",
		Rationale: "Low contention verified by load test",
	})

	result, err := QueryStatus(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Active Decisions") {
		t.Error("missing Active Decisions section")
	}
	if !strings.Contains(result, "Backlog") {
		t.Error("missing Backlog section for problems without linked artifacts")
	}
	if !strings.Contains(result, "Recent Notes") {
		t.Error("missing Recent Notes section")
	}
}

func TestQueryStatus_Empty(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	result, err := QueryStatus(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No artifacts found") {
		t.Error("expected 'No artifacts found' for empty DB")
	}
}

func TestQueryRelated_FindsByFile(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "NATS decision"},
		Body: "d",
	})
	store.SetAffectedFiles(ctx, "dec-001", []AffectedFile{{Path: "internal/events/producer.go"}})

	result, err := QueryRelated(ctx, store, "internal/events/producer.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "NATS decision") {
		t.Error("expected NATS decision in related results")
	}
}

func TestQueryRelated_NoResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	result, err := QueryRelated(ctx, store, "nonexistent.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No decisions found") {
		t.Error("expected 'No decisions found'")
	}
}

func TestQueryRelated_EmptyPath(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := QueryRelated(ctx, store, "")
	if err == nil {
		t.Error("expected error for empty file path")
	}
}
