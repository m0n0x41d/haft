package artifact

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestScanStale_FindsExpired(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	future := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Old decision", ValidUntil: past},
		Body: "expired",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-002", Kind: KindDecisionRecord, Title: "Fresh decision", ValidUntil: future},
		Body: "still valid",
	})

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stale, got %d", len(items))
	}
	if items[0].ID != "dec-001" {
		t.Errorf("expected dec-001, got %s", items[0].ID)
	}
	if items[0].DaysStale < 1 {
		t.Errorf("expected >0 days stale, got %d", items[0].DaysStale)
	}
}

func TestScanStale_NoneStale(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 stale, got %d", len(items))
	}
}

func TestWaiveDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Stale decision", ValidUntil: past, Status: StatusActive},
		Body: "# Decision\n\nOriginal content",
	})

	dec, err := WaiveDecision(ctx, store, quintDir, "dec-001", "Load test still valid at current traffic", "", "Recent test at 1000 req/s passed")
	if err != nil {
		t.Fatal(err)
	}

	if dec.Meta.Status != StatusActive {
		t.Errorf("status = %q, want active", dec.Meta.Status)
	}
	if dec.Meta.ValidUntil == past {
		t.Error("valid_until should have been extended")
	}
	if !strings.Contains(dec.Body, "## Waiver") {
		t.Error("waiver section not appended to body")
	}
}

func TestReopenDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Decision: NATS JetStream", Status: StatusActive, Context: "events"},
		Body: "# Decision\n\nOriginal content",
	})

	dec, newProb, err := ReopenDecision(ctx, store, quintDir, "dec-001", "Throughput approaching design limit")
	if err != nil {
		t.Fatal(err)
	}

	if dec.Meta.Status != StatusRefreshDue {
		t.Errorf("old decision status = %q, want refresh_due", dec.Meta.Status)
	}
	if newProb == nil {
		t.Fatal("expected new ProblemCard")
	}
	if newProb.Meta.Kind != KindProblemCard {
		t.Errorf("new artifact kind = %q, want ProblemCard", newProb.Meta.Kind)
	}
	if !strings.Contains(newProb.Meta.Title, "Revisit") {
		t.Errorf("new problem title should contain 'Revisit', got %q", newProb.Meta.Title)
	}
	if newProb.Meta.Context != "events" {
		t.Errorf("new problem context = %q, want events (inherited)", newProb.Meta.Context)
	}

	// Check link
	links, _ := store.GetLinks(ctx, newProb.Meta.ID)
	found := false
	for _, l := range links {
		if l.Ref == "dec-001" && l.Type == "revisits" {
			found = true
		}
	}
	if !found {
		t.Error("new problem should link to old decision with 'revisits'")
	}
}

func TestSupersedeDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Old", Status: StatusActive},
		Body: "old",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-002", Kind: KindDecisionRecord, Title: "New", Status: StatusActive},
		Body: "new",
	})

	dec, err := SupersedeDecision(ctx, store, quintDir, "dec-001", "dec-002", "Team doubled, need Kafka now")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Meta.Status != StatusSuperseded {
		t.Errorf("status = %q, want superseded", dec.Meta.Status)
	}
	if !strings.Contains(dec.Body, "Superseded") {
		t.Error("body should contain superseded section")
	}
}

func TestDeprecateDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Old", Status: StatusActive},
		Body: "old",
	})

	dec, err := DeprecateDecision(ctx, store, quintDir, "dec-001", "Feature removed entirely")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Meta.Status != StatusDeprecated {
		t.Errorf("status = %q, want deprecated", dec.Meta.Status)
	}
}

func TestCreateRefreshReport(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "D", Status: StatusActive},
		Body: "d",
	})

	report, err := CreateRefreshReport(ctx, store, quintDir, "dec-001", "waive", "Still valid", "Extended 90 days")
	if err != nil {
		t.Fatal(err)
	}
	if report.Meta.Kind != KindRefreshReport {
		t.Errorf("kind = %q", report.Meta.Kind)
	}
	if !strings.Contains(report.Body, "waive") {
		t.Error("report should mention action")
	}
}
