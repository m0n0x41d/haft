package fpf

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

type searchGoldenCase struct {
	Name               string   `json:"name"`
	Query              string   `json:"query"`
	RouteID            string   `json:"route_id"`
	SearchTier         string   `json:"search_tier"`
	ExpectedTier       string   `json:"expected_tier"`
	TopN               int      `json:"top_n"`
	MinimumHits        int      `json:"minimum_hits"`
	ExpectedPatternIDs []string `json:"expected_pattern_ids"`
}

func TestSearchSpec_GoldenQueries(t *testing.T) {
	cases := loadSearchGoldenCases(t)
	routes := loadGoldenRoutes(t)
	routeByID := indexRoutesByID(routes)
	patternIDs := collectRoutePatternIDs(routes)
	addGoldenCasePatternIDs(patternIDs, cases)

	chunks := loadGoldenSpecChunksForPatternIDs(t, patternIDs)
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	for _, test := range cases {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			results, err := SearchSpecWithOptions(db, test.Query, SpecSearchOptions{
				Limit: goldenSearchLimit(test),
				Tier:  test.SearchTier,
			})
			if err != nil {
				t.Fatalf("SearchSpec(%q) error: %v", test.Query, err)
			}

			topResults := truncateGoldenResults(results, test.TopN)
			hits := topGoldenHits(topResults, test.ExpectedPatternIDs)
			if len(hits) < goldenMinimumHits(test) {
				t.Fatalf(
					"SearchSpec(%q) top-%d hits = %d, want >= %d for %v; got %s",
					test.Query,
					test.TopN,
					len(hits),
					goldenMinimumHits(test),
					test.ExpectedPatternIDs,
					formatGoldenResults(topResults),
				)
			}

			if test.ExpectedTier != "" && !hasGoldenHitWithTier(hits, test.ExpectedTier) {
				t.Fatalf(
					"SearchSpec(%q) expected tier %q within top-%d for %v; got %s",
					test.Query,
					test.ExpectedTier,
					test.TopN,
					test.ExpectedPatternIDs,
					formatGoldenResults(topResults),
				)
			}

			if test.RouteID != "" {
				route, ok := routeByID[test.RouteID]
				if !ok {
					t.Fatalf("golden query references unknown route %q", test.RouteID)
				}
				if !hasGoldenHitWithReason(hits, route.Title) {
					t.Fatalf(
						"SearchSpec(%q) expected route reason %q within top-%d; got %s",
						test.Query,
						route.Title,
						test.TopN,
						formatGoldenResults(topResults),
					)
				}
			}
		})
	}
}

func loadSearchGoldenCases(t *testing.T) []searchGoldenCase {
	t.Helper()

	path := filepath.Join(testPackageDir(t), "testdata", "search_golden_queries.json")
	data := mustReadFile(t, path)

	cases := []searchGoldenCase{}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", path, err)
	}

	return cases
}

func addGoldenCasePatternIDs(patternIDs map[string]struct{}, cases []searchGoldenCase) {
	for _, test := range cases {
		for _, patternID := range test.ExpectedPatternIDs {
			patternIDs[patternID] = struct{}{}
		}
	}
}

func goldenSearchLimit(test searchGoldenCase) int {
	if test.TopN > 0 {
		return test.TopN
	}

	return DefaultSpecSearchLimit
}

func goldenMinimumHits(test searchGoldenCase) int {
	if test.MinimumHits > 0 {
		return test.MinimumHits
	}

	return 1
}

func truncateGoldenResults(results []SpecSearchResult, topN int) []SpecSearchResult {
	if topN <= 0 || len(results) <= topN {
		return results
	}

	return results[:topN]
}

func topGoldenHits(results []SpecSearchResult, expectedPatternIDs []string) []SpecSearchResult {
	hits := make([]SpecSearchResult, 0, len(expectedPatternIDs))
	for _, result := range results {
		if containsString(expectedPatternIDs, result.PatternID) {
			hits = append(hits, result)
		}
	}

	return hits
}

func hasGoldenHitWithTier(results []SpecSearchResult, tier string) bool {
	for _, result := range results {
		if result.Tier == tier {
			return true
		}
	}

	return false
}

func hasGoldenHitWithReason(results []SpecSearchResult, reason string) bool {
	for _, result := range results {
		if result.Reason == reason {
			return true
		}
	}

	return false
}

func formatGoldenResults(results []SpecSearchResult) string {
	if len(results) == 0 {
		return "[]"
	}

	formatted := make([]string, 0, len(results))
	for _, result := range results {
		formatted = append(formatted, fmt.Sprintf("%s[%s:%s]", result.PatternID, result.Tier, result.Reason))
	}

	return "[" + stringsJoin(formatted, ", ") + "]"
}

func stringsJoin(values []string, separator string) string {
	if len(values) == 0 {
		return ""
	}

	joined := values[0]
	for _, value := range values[1:] {
		joined += separator + value
	}

	return joined
}
