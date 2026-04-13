package fpf

import (
	"encoding/json"
	"path/filepath"
	"sort"
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
	chunks := loadGoldenSpecChunks(t)
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	routeByID := indexRoutesByID(routes)
	cases := loadHReasonPromptCases(t)
	coveredRoutes := make(map[string]struct{}, len(cases))

	for _, test := range cases {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			route, ok := routeByID[test.RouteID]
			if !ok {
				t.Fatalf("h-reason prompt references unknown route %q", test.RouteID)
			}
			coveredRoutes[test.RouteID] = struct{}{}

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

			searchLimit := routeGoldenSearchLimit(test.ExpectedPatternIDs)

			results, err := SearchSpec(db, test.Prompt, searchLimit)
			if err != nil {
				t.Fatalf("SearchSpec(%q) error: %v", test.Prompt, err)
			}

			topResults := truncateGoldenResults(results, searchLimit)
			hits := topGoldenHits(topResults, test.ExpectedPatternIDs)
			if len(hits) != len(test.ExpectedPatternIDs) {
				t.Fatalf(
					"SearchSpec(%q) top-%d hits = %d, want %d for %v; got %s",
					test.Prompt,
					searchLimit,
					len(hits),
					len(test.ExpectedPatternIDs),
					test.ExpectedPatternIDs,
					formatGoldenResults(topResults),
				)
			}

			routePatternIDs := resultPatternIDs(filterResultsByTier(results, SpecSearchTierRoute))
			if !hasPatternIDPrefix(routePatternIDs, test.ExpectedPatternIDs) {
				t.Fatalf("SearchSpec(%q) route-tier patterns = %v, want prefix %v", test.Prompt, routePatternIDs, test.ExpectedPatternIDs)
			}

			for _, hit := range hits {
				if hit.Tier != SpecSearchTierRoute {
					t.Fatalf("SearchSpec(%q) pattern %q tier = %q, want %q", test.Prompt, hit.PatternID, hit.Tier, SpecSearchTierRoute)
				}
				if hit.Reason != route.Title {
					t.Fatalf("SearchSpec(%q) pattern %q reason = %q, want %q", test.Prompt, hit.PatternID, hit.Reason, route.Title)
				}
			}
		})
	}

	if len(coveredRoutes) != len(routeByID) {
		missing := make([]string, 0, len(routeByID)-len(coveredRoutes))
		for routeID := range routeByID {
			if _, ok := coveredRoutes[routeID]; ok {
				continue
			}
			missing = append(missing, routeID)
		}
		sort.Strings(missing)
		t.Fatalf("h-reason prompt coverage incomplete: missing %v", missing)
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
