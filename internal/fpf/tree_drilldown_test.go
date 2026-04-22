package fpf

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
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
	if results[1].PatternID != "A.6" {
		t.Fatalf("expected parent section in second slot, got %#v", results[1])
	}
}

func TestDrillDownRouteSeedsStayAheadOfFTSLeaves(t *testing.T) {
	section := func(patternID string, parentPatternID string) drillDownSection {
		return drillDownSection{
			PatternID:       patternID,
			Heading:         patternID,
			Summary:         patternID,
			ParentPatternID: parentPatternID,
		}
	}

	branches := []drillDownBranch{
		{
			LeafPatternID: "A.6.3.CR",
			Path: []drillDownSection{
				section("A.6.3.CR", ""),
			},
			Score:     4,
			SeedOrder: 0,
			SeedTier:  SpecSearchTierPattern,
		},
		{
			LeafPatternID: "E.17.EFP",
			Path: []drillDownSection{
				section("E.17.EFP", ""),
			},
			Score:     3,
			SeedOrder: 1,
			SeedTier:  SpecSearchTierPattern,
		},
		{
			LeafPatternID: "A.6.3",
			Path: []drillDownSection{
				section("A.6.3", ""),
			},
			Score:     2,
			SeedOrder: 2,
			SeedTier:  SpecSearchTierPattern,
		},
		{
			LeafPatternID: "A.6.3.RT",
			Path: []drillDownSection{
				section("A.6.3.RT", ""),
			},
			Score:     1,
			SeedOrder: 3,
			SeedTier:  SpecSearchTierPattern,
		},
		{
			LeafPatternID: "A.6.3.CR:2",
			Path: []drillDownSection{
				section("A.6.3.CR:2", "A.6.3.CR"),
				section("A.6.3.CR", ""),
			},
			Score:     1000,
			SeedOrder: 4,
			SeedTier:  SpecSearchTierFTS,
		},
	}

	sortDrillDownBranches(branches)
	results := buildDrillDownResults(branches, 4)
	got := resultPatternIDs(results)
	want := []string{"A.6.3.CR", "E.17.EFP", "A.6.3", "A.6.3.RT"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("drill-down results = %v, want %v", got, want)
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

func TestSearchSpec_TreeModeGoldenQueriesBeatBaselineOnFullCorpus(t *testing.T) {
	cases := loadSearchGoldenCases(t)
	routes := loadGoldenRoutes(t)
	chunks := loadGoldenSpecChunks(t)
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	policy := treeGoldenEvaluationPolicy{}
	baseline := goldenRetrievalMetrics{Name: "deterministic", Total: len(cases)}
	tree := goldenRetrievalMetrics{Name: "tree", Total: len(cases)}
	baselineHierarchy := 0
	treeHierarchy := 0

	for _, test := range cases {
		baselineResults, err := SearchSpecWithOptions(db, test.Query, SpecSearchOptions{
			Limit: policy.limit(test),
			Tier:  test.SearchTier,
		})
		if err != nil {
			t.Fatalf("deterministic search for %q failed: %v", test.Query, err)
		}

		treeResults, err := SearchSpecWithOptions(db, test.Query, SpecSearchOptions{
			Limit: policy.limit(test),
			Tier:  test.SearchTier,
			Mode:  SpecSearchModeTree,
		})
		if err != nil {
			t.Fatalf("tree search for %q failed: %v", test.Query, err)
		}

		topBaseline := truncateGoldenResults(baselineResults, policy.limit(test))
		topTree := truncateGoldenResults(treeResults, policy.limit(test))
		baselineHits := topGoldenHits(topBaseline, test.ExpectedPatternIDs)
		treeHits := topGoldenHits(topTree, test.ExpectedPatternIDs)

		baseline.TotalHits += len(baselineHits)
		tree.TotalHits += len(treeHits)
		baselineHierarchy += countGoldenHierarchyLinks(topBaseline)
		treeHierarchy += countGoldenHierarchyLinks(topTree)

		if len(baselineHits) >= policy.minimumHits(test) {
			baseline.Successful++
		} else {
			baseline.Failures = append(baseline.Failures, test.Name+" -> "+formatGoldenResults(topBaseline))
		}
		if len(treeHits) >= policy.minimumHits(test) {
			tree.Successful++
		} else {
			tree.Failures = append(tree.Failures, test.Name+" -> "+formatGoldenResults(topTree))
		}
	}

	t.Logf("deterministic golden coverage: %d/%d cases, %d total expected hits, %d hierarchy links", baseline.Successful, baseline.Total, baseline.TotalHits, baselineHierarchy)
	t.Logf("tree golden coverage: %d/%d cases, %d total expected hits, %d hierarchy links", tree.Successful, tree.Total, tree.TotalHits, treeHierarchy)

	if baseline.Successful != len(cases) {
		t.Fatalf("deterministic comparison harness regressed: %s", baseline.failureSummary())
	}
	if tree.Successful != len(cases) {
		t.Fatalf("tree drill-down prototype missed curated cases: %s", tree.failureSummary())
	}
	if treeHierarchy <= baselineHierarchy {
		t.Fatalf("tree drill-down did not beat baseline on hierarchy retention: baseline=%d tree=%d", baselineHierarchy, treeHierarchy)
	}
}

func TestSearchSpec_TreeDrillDownSupplementalPathQueries(t *testing.T) {
	cases := loadTreeDrillDownGoldenCases(t)
	routes := loadGoldenRoutes(t)
	chunks := loadGoldenSpecChunks(t)
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

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

			if len(treeHits) == 0 {
				t.Fatalf(
					"tree search lost all expected path hits for %v; got %s",
					test.ExpectedPatternIDs,
					formatGoldenResults(treeResults),
				)
			}
			if len(baselineHits) == 0 {
				t.Fatalf(
					"baseline search unexpectedly lost all expected path hits for %v; got %s",
					test.ExpectedPatternIDs,
					formatGoldenResults(baselineResults),
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

type treeGoldenEvaluationPolicy struct{}

func (treeGoldenEvaluationPolicy) limit(test searchGoldenCase) int {
	if test.ExpectedTier == SpecSearchTierPattern {
		return 1
	}
	if test.TopN > 5 {
		return test.TopN
	}
	return 5
}

func (treeGoldenEvaluationPolicy) minimumHits(test searchGoldenCase) int {
	return goldenMinimumHits(test)
}

func countGoldenHierarchyLinks(results []SpecSearchResult) int {
	patternIDs := make([]string, 0, len(results))
	links := 0
	for _, result := range results {
		patternID := normalizePatternID(result.PatternID)
		if patternID == "" {
			continue
		}
		patternIDs = append(patternIDs, patternID)
	}

	for _, childPatternID := range patternIDs {
		for _, ancestorPatternID := range patternIDs {
			if isPatternAncestor(ancestorPatternID, childPatternID) {
				links++
				break
			}
		}
	}
	return links
}

func isPatternAncestor(ancestorPatternID string, childPatternID string) bool {
	if ancestorPatternID == "" || childPatternID == "" || ancestorPatternID == childPatternID {
		return false
	}
	if !strings.HasPrefix(childPatternID, ancestorPatternID) {
		return false
	}
	if len(childPatternID) <= len(ancestorPatternID) {
		return false
	}
	switch childPatternID[len(ancestorPatternID)] {
	case '.', ':':
		return true
	default:
		return false
	}
}
