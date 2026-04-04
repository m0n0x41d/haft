package fpf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
)

type routeGoldenCase struct {
	Name          string   `json:"name"`
	Query         string   `json:"query"`
	RouteID       string   `json:"route_id"`
	TopPatternIDs []string `json:"top_pattern_ids"`
}

func TestSearchSpec_RouteGoldenQueries(t *testing.T) {
	routes := loadRouteGoldenRoutes(t)
	chunks := buildRouteGoldenChunks(routes)
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

			results, err := SearchSpec(db, test.Query, len(test.TopPatternIDs))
			if err != nil {
				t.Fatalf("SearchSpec(%q) error: %v", test.Query, err)
			}

			gotPatternIDs := resultPatternIDs(results)
			if !reflect.DeepEqual(gotPatternIDs, test.TopPatternIDs) {
				t.Fatalf("SearchSpec(%q) top patterns = %v, want %v", test.Query, gotPatternIDs, test.TopPatternIDs)
			}

			for _, result := range results {
				if result.Tier != "route" {
					t.Fatalf("SearchSpec(%q) result %#v did not stay in route tier", test.Query, result)
				}
				if result.Reason != route.Title {
					t.Fatalf("SearchSpec(%q) reason = %q, want %q", test.Query, result.Reason, route.Title)
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

func loadRouteGoldenRoutes(t *testing.T) []Route {
	t.Helper()

	path := filepath.Join(testRepoRoot(t), ".context", "fpf-routes.json")
	routes, err := LoadRoutes(path)
	if err != nil {
		t.Fatalf("LoadRoutes(%q) failed: %v", path, err)
	}

	return routes
}

func buildRouteGoldenChunks(routes []Route) []SpecChunk {
	chunks := make([]SpecChunk, 0)
	seenPatternIDs := make(map[string]struct{})

	appendPattern := func(patternID string) {
		if patternID == "" {
			return
		}
		if _, ok := seenPatternIDs[patternID]; ok {
			return
		}
		seenPatternIDs[patternID] = struct{}{}
		chunks = append(chunks, SpecChunk{
			ID:        len(chunks),
			Heading:   patternID + " - Route golden fixture",
			Level:     2,
			Body:      "Route golden fixture body.",
			PatternID: patternID,
		})
	}

	for _, route := range routes {
		for _, patternID := range route.Chain {
			appendPattern(patternID)
		}
		for _, patternID := range route.Core {
			appendPattern(patternID)
		}
	}

	return chunks
}

func indexRoutesByID(routes []Route) map[string]Route {
	indexed := make(map[string]Route, len(routes))
	for _, route := range routes {
		indexed[route.ID] = route
	}

	return indexed
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) failed: %v", path, err)
	}

	return data
}

func testPackageDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Dir(filename)
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	return filepath.Clean(filepath.Join(testPackageDir(t), "..", ".."))
}
