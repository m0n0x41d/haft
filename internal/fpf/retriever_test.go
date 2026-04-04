package fpf

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRetrieveSpec_UsesStructuredSnippetByDefault(t *testing.T) {
	_, db, cleanup := buildRetrievalTestIndex(t)
	defer cleanup()

	result, err := RetrieveSpec(db, SpecRetrievalRequest{
		Query: "A.6",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("RetrieveSpec returned error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 retrieval result, got %d", len(result.Results))
	}

	hit := result.Results[0]
	if hit.PatternID != "A.6" {
		t.Fatalf("expected A.6 hit, got %#v", hit)
	}
	if hit.Tier != SpecSearchTierPattern {
		t.Fatalf("expected pattern tier, got %q", hit.Tier)
	}
	if hit.Reason != "exact pattern id" {
		t.Fatalf("expected pattern-id reason, got %q", hit.Reason)
	}
	if !strings.Contains(hit.Summary, "Boundary routing keeps claims") {
		t.Fatalf("expected summary to round-trip, got %q", hit.Summary)
	}
	if strings.Contains(hit.Content, "TAIL-MARKER") {
		t.Fatalf("expected default retrieval to keep snippet-sized content, got %q", hit.Content)
	}
}

func TestRetrieveSpec_HydratesFullSectionContent(t *testing.T) {
	_, db, cleanup := buildRetrievalTestIndex(t)
	defer cleanup()

	result, err := RetrieveSpec(db, SpecRetrievalRequest{
		Query: "A.6",
		Limit: 1,
		Full:  true,
	})
	if err != nil {
		t.Fatalf("RetrieveSpec returned error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 retrieval result, got %d", len(result.Results))
	}
	if !strings.Contains(result.Results[0].Content, "TAIL-MARKER") {
		t.Fatalf("expected full retrieval to include the complete section body, got %q", result.Results[0].Content)
	}
}

func buildRetrievalTestIndex(t *testing.T) (string, *sql.DB, func()) {
	t.Helper()

	body := "Boundary routing keeps claims on the right layer. " + strings.Repeat("Boundary routing body ", 20) + "TAIL-MARKER"
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      body,
			PatternID: "A.6",
			Keywords:  []string{"boundary", "routing"},
			Queries:   []string{"How do I route boundary statements?"},
		},
		{
			ID:        1,
			Heading:   "A.6.B - Boundary Norm Square",
			Level:     2,
			Body:      "Norm square body",
			PatternID: "A.6.B",
			Keywords:  []string{"boundary", "deontics"},
			Queries:   []string{"What is the Boundary Norm Square?"},
		},
	}
	routes := []Route{{
		ID:          "boundary-discipline",
		Title:       "Boundary discipline and routing",
		Description: "Boundary statements and routing",
		Matchers:    []string{"boundary", "routing"},
		Core:        []string{"A.6", "A.6.B"},
		Chain:       []string{"A.6", "A.6.B"},
	}}

	return buildIndexWithChunksAndRoutes(t, chunks, routes, false)
}
