package fpf

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

type treeDrillDownGoldenCase struct {
	Name               string   `json:"name"`
	Query              string   `json:"query"`
	TopN               int      `json:"top_n"`
	ExpectedPatternIDs []string `json:"expected_pattern_ids"`
}

func TestSearchSpecWithOptions_TreeModeReturnsLeafBeforeAncestors(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      "Boundary routing keeps claims on the right layer.",
			PatternID: "A.6",
			Keywords:  []string{"boundary", "routing"},
			Queries:   []string{"How do I route boundary statements?"},
		},
		{
			ID:              1,
			Heading:         "A.6.B - Boundary Norm Square",
			Level:           3,
			Body:            "Deontic and admissibility language stay in the norm square.",
			PatternID:       "A.6.B",
			ParentPatternID: "A.6",
			Keywords:        []string{"boundary", "deontics", "admissibility"},
			Queries:         []string{"What is the Boundary Norm Square?"},
		},
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	results, err := SearchSpecWithOptions(db, "boundary deontics admissibility", SpecSearchOptions{
		Limit: 3,
		Mode:  SpecSearchModeTree,
	})
	if err != nil {
		t.Fatalf("SearchSpecWithOptions(tree) error: %v", err)
	}

	got := resultPatternIDs(results)
	want := []string{"A.6.B", "A.6"}
	if !reflect.DeepEqual(got[:2], want) {
		t.Fatalf("tree mode path = %v, want prefix %v", got, want)
	}
	if results[0].Tier != SpecSearchTierDrillDown {
		t.Fatalf("expected drilldown tier, got %#v", results[0])
	}
	if results[1].Reason != "tree drill-down ancestor of A.6.B" {
		t.Fatalf("unexpected ancestor reason %#v", results[1])
	}
}

func TestSearchSpecWithOptions_DrillDownTierFilterKeepsOnlyExperimentalHits(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      "Boundary routing keeps claims on the right layer.",
			PatternID: "A.6",
			Keywords:  []string{"boundary", "routing"},
		},
		{
			ID:              1,
			Heading:         "A.6.B - Boundary Norm Square",
			Level:           3,
			Body:            "Deontic and admissibility language stay in the norm square.",
			PatternID:       "A.6.B",
			ParentPatternID: "A.6",
			Keywords:        []string{"boundary", "deontics", "admissibility"},
		},
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	results, err := SearchSpecWithOptions(db, "boundary deontics", SpecSearchOptions{
		Limit: 3,
		Tier:  SpecSearchTierDrillDown,
		Mode:  SpecSearchModeTree,
	})
	if err != nil {
		t.Fatalf("SearchSpecWithOptions(drilldown tier) error: %v", err)
	}

	for _, result := range results {
		if result.Tier != SpecSearchTierDrillDown {
			t.Fatalf("unexpected non-drilldown result %#v", result)
		}
	}
}

func TestSearchSpec_TreeDrillDownGoldenQueriesOutperformBaseline(t *testing.T) {
	cases := loadTreeDrillDownGoldenCases(t)
	routes := loadGoldenRoutes(t)
	patternIDs := collectRoutePatternIDs(routes)
	addTreeDrillDownPatternIDs(patternIDs, cases)

	chunks := loadGoldenSpecChunksForPatternIDs(t, patternIDs)
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	baselineHitCount := 0
	treeHitCount := 0

	for _, test := range cases {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			baselineResults, err := SearchSpecWithOptions(db, test.Query, SpecSearchOptions{
				Limit: test.TopN,
			})
			if err != nil {
				t.Fatalf("baseline search error: %v", err)
			}

			treeResults, err := SearchSpecWithOptions(db, test.Query, SpecSearchOptions{
				Limit: test.TopN,
				Mode:  SpecSearchModeTree,
			})
			if err != nil {
				t.Fatalf("tree search error: %v", err)
			}

			baselineHits := topGoldenHits(truncateGoldenResults(baselineResults, test.TopN), test.ExpectedPatternIDs)
			treeHits := topGoldenHits(truncateGoldenResults(treeResults, test.TopN), test.ExpectedPatternIDs)

			baselineHitCount += len(baselineHits)
			treeHitCount += len(treeHits)

			if len(treeHits) == 0 {
				t.Fatalf(
					"tree search lost all expected path hits for %v; got %s",
					test.ExpectedPatternIDs,
					formatGoldenResults(treeResults),
				)
			}
			if !hasGoldenHitWithTier(treeResults, SpecSearchTierDrillDown) {
				t.Fatalf("expected drilldown tier in tree results, got %s", formatGoldenResults(treeResults))
			}
			if len(treeHits) < len(baselineHits) {
				t.Fatalf(
					"tree search regressed against baseline for %v; baseline=%s tree=%s",
					test.ExpectedPatternIDs,
					formatGoldenResults(baselineResults),
					formatGoldenResults(treeResults),
				)
			}
		})
	}

	if treeHitCount <= baselineHitCount {
		t.Fatalf("tree drill-down did not outperform baseline: baseline=%d tree=%d", baselineHitCount, treeHitCount)
	}
}

func loadTreeDrillDownGoldenCases(t *testing.T) []treeDrillDownGoldenCase {
	t.Helper()

	path := filepath.Join(testPackageDir(t), "testdata", "tree_drilldown_golden_queries.json")
	data := mustReadFile(t, path)

	cases := []treeDrillDownGoldenCase{}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", path, err)
	}

	return cases
}

func addTreeDrillDownPatternIDs(patternIDs map[string]struct{}, cases []treeDrillDownGoldenCase) {
	for _, test := range cases {
		for _, patternID := range test.ExpectedPatternIDs {
			patternIDs[patternID] = struct{}{}
		}
	}
}
