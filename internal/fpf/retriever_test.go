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
	if hit.Provenance.ProfileID == "" {
		t.Fatal("expected retrieval provenance profile id")
	}
	if hit.Provenance.SourceKind != specRetrievalSourceKindSection {
		t.Fatalf("source kind = %q, want %q", hit.Provenance.SourceKind, specRetrievalSourceKindSection)
	}
	if hit.Provenance.Normativity != specRetrievalNormativitySource {
		t.Fatalf("normativity = %q, want %q", hit.Provenance.Normativity, specRetrievalNormativitySource)
	}
	if hit.Provenance.RetrievalMode != SpecRetrievalModeFTS {
		t.Fatalf("retrieval mode = %q, want fts", hit.Provenance.RetrievalMode)
	}
}

func TestRetrieveSpec_ProvenanceCarriesEditionValidityAndRouteNormativity(t *testing.T) {
	dbPath, db, cleanup := buildRetrievalTestIndex(t)
	defer cleanup()

	if err := SetSpecMetaEntries(dbPath, map[string]string{
		"fpf_commit":          "abc1234",
		"source_edition":      "FPF-2026-06-18",
		"profile_valid_until": "2026-07-18",
		"spec_path":           "data/FPF/FPF-Spec.md",
		"schema_version":      SpecIndexSchemaVersion,
	}); err != nil {
		t.Fatalf("SetSpecMetaEntries failed: %v", err)
	}

	result, err := RetrieveSpec(db, SpecRetrievalRequest{
		Query: "boundary",
		Limit: 1,
		Tier:  SpecSearchTierRoute,
	})
	if err != nil {
		t.Fatalf("RetrieveSpec returned error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 route result, got %d", len(result.Results))
	}

	provenance := result.Results[0].Provenance
	if provenance.SourceKind != specRetrievalSourceKindRouteCarrier {
		t.Fatalf("source kind = %q, want %q", provenance.SourceKind, specRetrievalSourceKindRouteCarrier)
	}
	if provenance.SourceEdition != "FPF-2026-06-18" {
		t.Fatalf("source edition = %q", provenance.SourceEdition)
	}
	if provenance.SourceHash != "abc1234" {
		t.Fatalf("source hash = %q", provenance.SourceHash)
	}
	if provenance.ProfileValidity != "valid_until=2026-07-18" {
		t.Fatalf("profile validity = %q", provenance.ProfileValidity)
	}
	if provenance.Normativity != specRetrievalNormativityRouteCarrier {
		t.Fatalf("normativity = %q, want %q", provenance.Normativity, specRetrievalNormativityRouteCarrier)
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

// The opt-in "semantic" retrieval mode was retired in favor of hybrid-by-default
// (the local-sidecar baked-vector path injected via HybridSearch). An unknown or
// retired mode must fall back to the deterministic search, never error.
func TestRetrieveSpec_RetiredSemanticModeFallsBackToDeterministic(t *testing.T) {
	_, db, cleanup := buildRetrievalTestIndex(t)
	defer cleanup()

	if _, err := RetrieveSpec(db, SpecRetrievalRequest{
		Query: "boundary contract unpacking",
		Limit: 2,
		Mode:  SpecRetrievalModeSemantic,
	}); err != nil {
		t.Fatalf("a retired/unknown mode must fall back to deterministic search, not error: %v", err)
	}
}

func TestRetrieveSpec_TreeModeUsesDrillDownTier(t *testing.T) {
	_, db, cleanup := buildRetrievalTestIndex(t)
	defer cleanup()

	result, err := RetrieveSpec(db, SpecRetrievalRequest{
		Query: "boundary deontics",
		Limit: 3,
		Mode:  SpecSearchModeTree,
	})
	if err != nil {
		t.Fatalf("RetrieveSpec returned error: %v", err)
	}

	if len(result.Results) < 2 {
		t.Fatalf("expected tree retrieval path, got %#v", result.Results)
	}
	if result.Results[0].Tier != SpecSearchTierDrillDown {
		t.Fatalf("expected drilldown tier, got %q", result.Results[0].Tier)
	}
	if result.Results[0].PatternID != "A.6.B" {
		t.Fatalf("expected leaf-first tree result, got %#v", result.Results[0])
	}
}

func TestRetrieveSpec_ExplicitControlsBypassHybrid(t *testing.T) {
	_, db, cleanup := buildRetrievalTestIndex(t)
	defer cleanup()

	tests := []struct {
		name string
		req  SpecRetrievalRequest
	}{
		{
			name: "fts retrieval mode",
			req: SpecRetrievalRequest{
				Query: "boundary",
				Limit: 2,
				Mode:  SpecRetrievalModeFTS,
			},
		},
		{
			name: "tree search mode",
			req: SpecRetrievalRequest{
				Query: "boundary deontics",
				Limit: 3,
				Mode:  SpecSearchModeTree,
			},
		},
		{
			name: "tier filter",
			req: SpecRetrievalRequest{
				Query: "boundary",
				Limit: 2,
				Tier:  SpecSearchTierRoute,
			},
		},
	}

	for _, tt := range tests {
		calls := 0
		req := tt.req
		req.HybridSearch = func(_ *sql.DB, _ string, _ int) ([]SpecSearchResult, error) {
			calls++
			return []SpecSearchResult{{PatternID: "HYBRID-SENTINEL", Heading: "wrong"}}, nil
		}

		if _, err := RetrieveSpec(db, req); err != nil {
			t.Fatalf("%s: RetrieveSpec returned error: %v", tt.name, err)
		}
		if calls != 0 {
			t.Fatalf("%s: explicit controls should bypass hybrid, calls = %d", tt.name, calls)
		}
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
			ID:              1,
			Heading:         "A.6.B - Boundary Norm Square",
			Level:           2,
			Body:            "Norm square body",
			PatternID:       "A.6.B",
			ParentPatternID: "A.6",
			Keywords:        []string{"boundary", "deontics"},
			Queries:         []string{"What is the Boundary Norm Square?"},
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
