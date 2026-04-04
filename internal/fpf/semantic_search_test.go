package fpf

import (
	"fmt"
	"testing"
)

func TestSearchSpecSemantically_ExactPatternQueryStaysExact(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpecSemantically(db, "a.6:", SemanticSearchOptions{Limit: 3})
	if err != nil {
		t.Fatalf("SearchSpecSemantically returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected one exact-pattern result, got %d", len(results))
	}
	if results[0].PatternID != "A.6" {
		t.Fatalf("expected A.6 exact hit, got %#v", results[0])
	}
	if results[0].Tier != SpecSearchTierPattern {
		t.Fatalf("expected exact pattern tier, got %q", results[0].Tier)
	}
}

func TestSearchSpecSemantically_GoldenCoverage(t *testing.T) {
	cases := loadSearchGoldenCases(t)
	routes := loadGoldenRoutes(t)
	patternIDs := collectRoutePatternIDs(routes)
	addGoldenCasePatternIDs(patternIDs, cases)

	chunks := loadGoldenSpecChunksForPatternIDs(t, patternIDs)
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	policy := semanticGoldenEvaluationPolicy{}
	deterministic := evaluateGoldenRetrieval(t, "deterministic", cases, func(test searchGoldenCase) ([]SpecSearchResult, error) {
		return SearchSpecWithOptions(db, test.Query, SpecSearchOptions{
			Limit: policy.limit(test),
		})
	}, policy)
	semantic := evaluateGoldenRetrieval(t, "semantic", cases, func(test searchGoldenCase) ([]SpecSearchResult, error) {
		return SearchSpecSemantically(db, test.Query, SemanticSearchOptions{
			Limit: policy.limit(test),
		})
	}, policy)

	t.Logf("deterministic golden coverage: %d/%d cases, %d total expected hits", deterministic.Successful, deterministic.Total, deterministic.TotalHits)
	t.Logf("semantic golden coverage: %d/%d cases, %d total expected hits", semantic.Successful, semantic.Total, semantic.TotalHits)

	if deterministic.Successful != len(cases) {
		t.Fatalf("deterministic comparison harness regressed: %s", deterministic.failureSummary())
	}
	if semantic.Successful != len(cases) {
		t.Fatalf("semantic prototype missed curated cases: %s", semantic.failureSummary())
	}
}

type semanticGoldenEvaluationPolicy struct{}

func (semanticGoldenEvaluationPolicy) limit(test searchGoldenCase) int {
	if test.TopN > 5 {
		return test.TopN
	}
	if test.ExpectedTier == SpecSearchTierPattern {
		return 1
	}
	return 5
}

func (semanticGoldenEvaluationPolicy) minimumHits(test searchGoldenCase) int {
	if test.ExpectedTier == SpecSearchTierPattern {
		return 1
	}
	return 1
}

type goldenRetrievalMetrics struct {
	Name       string
	Total      int
	Successful int
	TotalHits  int
	Failures   []string
}

func (metrics goldenRetrievalMetrics) failureSummary() string {
	if len(metrics.Failures) == 0 {
		return "none"
	}
	return fmt.Sprintf("%v", metrics.Failures)
}

func evaluateGoldenRetrieval(
	t *testing.T,
	name string,
	cases []searchGoldenCase,
	search func(searchGoldenCase) ([]SpecSearchResult, error),
	policy semanticGoldenEvaluationPolicy,
) goldenRetrievalMetrics {
	t.Helper()

	metrics := goldenRetrievalMetrics{
		Name:  name,
		Total: len(cases),
	}

	for _, test := range cases {
		results, err := search(test)
		if err != nil {
			t.Fatalf("%s search for %q failed: %v", name, test.Query, err)
		}

		topResults := truncateGoldenResults(results, policy.limit(test))
		hits := topGoldenHits(topResults, test.ExpectedPatternIDs)
		metrics.TotalHits += len(hits)

		if len(hits) >= policy.minimumHits(test) {
			metrics.Successful++
			continue
		}

		failure := fmt.Sprintf("%s -> %s", test.Name, formatGoldenResults(topResults))
		metrics.Failures = append(metrics.Failures, failure)
	}

	return metrics
}
