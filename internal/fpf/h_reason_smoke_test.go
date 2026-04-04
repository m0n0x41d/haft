package fpf

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

type hReasonPromptCase struct {
	Name               string   `json:"name"`
	Prompt             string   `json:"prompt"`
	RouteID            string   `json:"route_id"`
	ExpectedPatternIDs []string `json:"expected_pattern_ids"`
}

func TestSearchSpec_HReasonPromptSmokes(t *testing.T) {
	routes := loadGoldenRoutes(t)
	chunks := loadGoldenSpecChunksForPatternIDs(t, collectRoutePatternIDs(routes))
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	routeByID := indexRoutesByID(routes)
	cases := loadHReasonPromptCases(t)

	for _, test := range cases {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			route, ok := routeByID[test.RouteID]
			if !ok {
				t.Fatalf("h-reason prompt references unknown route %q", test.RouteID)
			}

			classified, err := classifyRoute(db, test.Prompt)
			if err != nil {
				t.Fatalf("classifyRoute(%q) error: %v", test.Prompt, err)
			}
			if classified == nil {
				t.Fatalf("classifyRoute(%q) returned no route", test.Prompt)
			}
			if classified.ID != test.RouteID {
				t.Fatalf("classifyRoute(%q) route = %q, want %q", test.Prompt, classified.ID, test.RouteID)
			}

			results, err := SearchSpec(db, test.Prompt, routeGoldenSearchLimit(test.ExpectedPatternIDs))
			if err != nil {
				t.Fatalf("SearchSpec(%q) error: %v", test.Prompt, err)
			}

			topPatternIDs := resultPatternIDs(results)
			if !hasPatternIDPrefix(topPatternIDs, test.ExpectedPatternIDs) {
				t.Fatalf("SearchSpec(%q) top patterns = %v, want prefix %v", test.Prompt, topPatternIDs, test.ExpectedPatternIDs)
			}

			routePatternIDs := resultPatternIDs(filterResultsByTier(results, SpecSearchTierRoute))
			if !hasPatternIDPrefix(routePatternIDs, test.ExpectedPatternIDs) {
				t.Fatalf("SearchSpec(%q) route-tier patterns = %v, want prefix %v", test.Prompt, routePatternIDs, test.ExpectedPatternIDs)
			}

			for _, patternID := range test.ExpectedPatternIDs {
				result := findResultByPatternID(results, patternID)
				if result == nil {
					t.Fatalf("SearchSpec(%q) missing expected pattern %q in %v", test.Prompt, patternID, resultPatternIDs(results))
				}
				if result.Tier != SpecSearchTierRoute {
					t.Fatalf("SearchSpec(%q) pattern %q tier = %q, want %q", test.Prompt, patternID, result.Tier, SpecSearchTierRoute)
				}
				if result.Reason != route.Title {
					t.Fatalf("SearchSpec(%q) pattern %q reason = %q, want %q", test.Prompt, patternID, result.Reason, route.Title)
				}
			}
		})
	}
}

func loadHReasonPromptCases(t *testing.T) []hReasonPromptCase {
	t.Helper()

	path := filepath.Join(testPackageDir(t), "testdata", "h_reason_prompt_queries.json")
	data := mustReadFile(t, path)

	cases := []hReasonPromptCase{}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", path, err)
	}

	return cases
}
