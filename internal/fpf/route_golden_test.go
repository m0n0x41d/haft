package fpf

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type routeGoldenCase struct {
	Name               string   `json:"name"`
	Query              string   `json:"query"`
	RouteID            string   `json:"route_id"`
	ExpectedPatternIDs []string `json:"expected_pattern_ids"`
}

func TestSearchSpec_RouteGoldenQueries(t *testing.T) {
	routes := loadGoldenRoutes(t)
	chunks := loadGoldenSpecChunksForPatternIDs(t, collectRoutePatternIDs(routes))
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	routeByID := indexRoutesByID(routes)
	cases := loadRouteGoldenCases(t)
	coveredRoutes := make(map[string]struct{}, len(cases))

	for _, test := range cases {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			route, ok := routeByID[test.RouteID]
			if !ok {
				t.Fatalf("golden query references unknown route %q", test.RouteID)
			}
			coveredRoutes[test.RouteID] = struct{}{}

			classified, err := classifyRoute(db, test.Query)
			if err != nil {
				t.Fatalf("classifyRoute(%q) error: %v", test.Query, err)
			}
			if classified == nil {
				t.Fatalf("classifyRoute(%q) returned no route", test.Query)
			}
			if classified.ID != test.RouteID {
				t.Fatalf("classifyRoute(%q) route = %q, want %q", test.Query, classified.ID, test.RouteID)
			}

			results, err := SearchSpec(db, test.Query, routeGoldenSearchLimit(test.ExpectedPatternIDs))
			if err != nil {
				t.Fatalf("SearchSpec(%q) error: %v", test.Query, err)
			}

			topPatternIDs := resultPatternIDs(results)
			if !hasPatternIDPrefix(topPatternIDs, test.ExpectedPatternIDs) {
				t.Fatalf("SearchSpec(%q) top patterns = %v, want prefix %v", test.Query, topPatternIDs, test.ExpectedPatternIDs)
			}

			routePatternIDs := resultPatternIDs(filterResultsByTier(results, "route"))
			if !hasPatternIDPrefix(routePatternIDs, test.ExpectedPatternIDs) {
				t.Fatalf("SearchSpec(%q) route-tier patterns = %v, want prefix %v", test.Query, routePatternIDs, test.ExpectedPatternIDs)
			}

			for _, patternID := range test.ExpectedPatternIDs {
				result := findResultByPatternID(results, patternID)
				if result == nil {
					t.Fatalf("SearchSpec(%q) missing expected pattern %q in %v", test.Query, patternID, resultPatternIDs(results))
				}
				if result.Tier != "route" {
					t.Fatalf("SearchSpec(%q) pattern %q tier = %q, want route", test.Query, patternID, result.Tier)
				}
				if result.Reason != route.Title {
					t.Fatalf("SearchSpec(%q) pattern %q reason = %q, want %q", test.Query, patternID, result.Reason, route.Title)
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
		t.Fatalf("golden route coverage incomplete: missing %v", missing)
	}
}

func loadRouteGoldenCases(t *testing.T) []routeGoldenCase {
	t.Helper()

	path := filepath.Join(testPackageDir(t), "testdata", "route_golden_queries.json")
	data := mustReadFile(t, path)

	cases := []routeGoldenCase{}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", path, err)
	}

	return cases
}

func indexRoutesByID(routes []Route) map[string]Route {
	indexed := make(map[string]Route, len(routes))
	for _, route := range routes {
		indexed[route.ID] = route
	}

	return indexed
}

func routeGoldenSearchLimit(expectedPatternIDs []string) int {
	limit := len(expectedPatternIDs) + 4
	if limit < 8 {
		return 8
	}

	return limit
}

func hasPatternIDPrefix(patternIDs []string, prefix []string) bool {
	if len(patternIDs) < len(prefix) {
		return false
	}

	return reflect.DeepEqual(patternIDs[:len(prefix)], prefix)
}
