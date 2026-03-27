package artifact

import (
	"context"
	"testing"
	"time"
)

func TestFetchSearchResults_FindsResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "NATS JetStream for events"},
		Body: "Selected NATS over Kafka for domain event infrastructure",
	})

	results, err := FetchSearchResults(ctx, store, "NATS events", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	found := false
	for _, a := range results {
		if a.Meta.Title == "NATS JetStream for events" {
			found = true
		}
	}
	if !found {
		t.Error("expected NATS JetStream artifact in results")
	}
}

func TestFetchSearchResults_NoResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	results, err := FetchSearchResults(ctx, store, "nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFetchSearchResults_EmptyQuery(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := FetchSearchResults(ctx, store, "", 10)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestFetchStatusData_Dashboard(t *testing.T) {
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

	data, err := FetchStatusData(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}

	hasPendingOrShipped := len(data.PendingDecisions) > 0 || len(data.ShippedDecisions) > 0
	if !hasPendingOrShipped {
		t.Error("missing pending or shipped decisions")
	}
	if len(data.BacklogProblems) == 0 {
		t.Error("missing backlog problems")
	}
	if len(data.RecentNotes) == 0 {
		t.Error("missing recent notes")
	}
}

func TestFetchStatusData_Empty(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	data, err := FetchStatusData(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}
	hasAny := len(data.PendingDecisions) > 0 ||
		len(data.ShippedDecisions) > 0 ||
		len(data.StaleItems) > 0 ||
		len(data.InProgressProblems) > 0 ||
		len(data.BacklogProblems) > 0 ||
		len(data.AddressedProblems) > 0 ||
		len(data.RecentNotes) > 0
	if hasAny {
		t.Error("expected empty status data for empty DB")
	}
}

func TestFetchRelatedArtifacts_FindsByFile(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "NATS decision"},
		Body: "d",
	})
	store.SetAffectedFiles(ctx, "dec-001", []AffectedFile{{Path: "internal/events/producer.go"}})

	results, err := FetchRelatedArtifacts(ctx, store, "internal/events/producer.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	found := false
	for _, a := range results {
		if a.Meta.Title == "NATS decision" {
			found = true
		}
	}
	if !found {
		t.Error("expected NATS decision in related results")
	}
}

func TestFetchRelatedArtifacts_NoResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	results, err := FetchRelatedArtifacts(ctx, store, "nonexistent.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFetchRelatedArtifacts_EmptyPath(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := FetchRelatedArtifacts(ctx, store, "")
	if err == nil {
		t.Error("expected error for empty file path")
	}
}
