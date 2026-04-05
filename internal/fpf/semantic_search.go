package fpf

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	SpecSearchTierSemantic           = "semantic"
	defaultSemanticMinimumSimilarity = 0.18
	maxSemanticRouteCandidates       = 1
)

// SemanticSearchOptions controls the optional embedding-backed retrieval path.
type SemanticSearchOptions struct {
	Limit            int
	MinScore         float64
	ArtifactPath     string
	Context          context.Context
	Embedder         SemanticEmbedder
	EmbedderFactory  func() (SemanticEmbedder, error)
	DisableRouteSeed bool
}

type semanticScoredDocument struct {
	PatternID string
	Score     float64
}

type semanticScoredRoute struct {
	RouteID string
	Score   float64
}

// SearchSpecSemantically runs the explicit embedding-backed retrieval
// experiment. The supported deterministic search stack remains the default.
func SearchSpecSemantically(
	db *sql.DB,
	query string,
	options SemanticSearchOptions,
) ([]SpecSearchResult, error) {
	options = normalizeSemanticSearchOptions(options)
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	resultsByKey := make(map[string]SpecSearchResult)
	appendResults := func(results []SpecSearchResult) {
		for _, result := range results {
			key := result.PatternID + "|" + result.Heading
			existing, ok := resultsByKey[key]
			if ok && existing.Rank <= result.Rank {
				continue
			}
			resultsByKey[key] = result
		}
	}

	patternID := extractPatternID(query)
	if patternID != "" {
		result, err := searchByPatternID(db, patternID)
		if err == nil {
			appendResults([]SpecSearchResult{result})
		}
	}
	if len(resultsByKey) >= options.Limit || semanticPatternOnlyQuery(query) {
		return sortedSemanticResults(resultsByKey, options.Limit), nil
	}

	embedder, err := resolveSemanticEmbedder(options)
	if err != nil {
		return nil, err
	}

	artifact, err := LoadSemanticArtifact(options.ArtifactPath)
	if err != nil {
		return nil, err
	}
	if err := validateSemanticArtifact(db, artifact, embedder.Descriptor()); err != nil {
		return nil, err
	}

	queryVector, err := embedSemanticQuery(options.Context, embedder, query)
	if err != nil {
		return nil, err
	}
	if len(queryVector) == 0 {
		return sortedSemanticResults(resultsByKey, options.Limit), nil
	}

	routes, err := loadIndexedRoutes(db)
	if err != nil {
		return nil, err
	}
	routeResults, err := searchSemanticRoutes(
		db,
		routes,
		artifact,
		query,
		queryVector,
		options.MinScore,
		options.DisableRouteSeed,
		embedder.Descriptor(),
	)
	if err != nil {
		return nil, err
	}
	appendResults(routeResults)

	scoredDocuments := scoreSemanticArtifactDocuments(
		artifact.Documents,
		queryVector,
		options.MinScore,
	)
	scoredDocuments = filterSemanticDocuments(scoredDocuments, resultPatternSet(resultsByKey))
	scoredDocuments = truncateSemanticDocuments(scoredDocuments, options.Limit-len(resultsByKey))

	documentResults := buildSemanticDocumentResults(db, scoredDocuments, embedder.Descriptor())
	appendResults(documentResults)

	return sortedSemanticResults(resultsByKey, options.Limit), nil
}

func normalizeSemanticSearchOptions(options SemanticSearchOptions) SemanticSearchOptions {
	if options.Limit <= 0 {
		options.Limit = DefaultSpecSearchLimit
	}
	if options.MinScore <= 0 {
		options.MinScore = defaultSemanticMinimumSimilarity
	}
	if options.Context == nil {
		options.Context = context.Background()
	}
	return options
}

func semanticPatternOnlyQuery(query string) bool {
	normalized := normalizePatternID(query)
	return normalized != "" && normalized == extractPatternID(query)
}

func resolveSemanticEmbedder(options SemanticSearchOptions) (SemanticEmbedder, error) {
	if options.Embedder != nil {
		return options.Embedder, nil
	}
	if options.EmbedderFactory != nil {
		embedder, err := options.EmbedderFactory()
		if err != nil {
			return nil, err
		}
		if embedder != nil {
			return embedder, nil
		}
	}
	return nil, fmt.Errorf(
		"semantic embedder not configured: build an artifact with `haft fpf semantic-index` and provide embedding credentials",
	)
}

func embedSemanticQuery(
	ctx context.Context,
	embedder SemanticEmbedder,
	query string,
) ([]float32, error) {
	vectors, err := embedder.EmbedTexts(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed semantic query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("embed semantic query: got %d vectors, want 1", len(vectors))
	}
	return normalizeEmbeddingVector(vectors[0]), nil
}

func searchSemanticRoutes(
	db *sql.DB,
	routes []Route,
	artifact SemanticArtifact,
	query string,
	queryVector []float32,
	minScore float64,
	disableRouteSeed bool,
	descriptor SemanticEmbedderDescriptor,
) ([]SpecSearchResult, error) {
	if !disableRouteSeed {
		if route, err := classifyRoute(db, query); err == nil && route != nil {
			return buildSemanticSeedRouteResults(db, *route, descriptor)
		}
	}

	routeByID := semanticRouteIndexByID(routes)
	scored := scoreSemanticArtifactRoutes(artifact.Routes, queryVector, minScore)
	scored = truncateSemanticRoutes(scored, maxSemanticRouteCandidates)

	return buildSemanticRouteResults(db, routeByID, scored, descriptor)
}

func semanticRouteIndexByID(routes []Route) map[string]Route {
	indexed := make(map[string]Route, len(routes))
	for _, route := range routes {
		indexed[route.ID] = route
	}
	return indexed
}

func buildSemanticSeedRouteResults(
	db *sql.DB,
	route Route,
	descriptor SemanticEmbedderDescriptor,
) ([]SpecSearchResult, error) {
	return buildSemanticRouteResultsForRoute(db, route, func(index int, result SpecSearchResult) SpecSearchResult {
		result.Rank = -600 + float64(index)
		result.Tier = SpecSearchTierSemantic
		result.Reason = fmt.Sprintf(
			"semantic route seed %s via deterministic classifier (%s/%s)",
			route.Title,
			descriptor.Provider,
			descriptor.Model,
		)
		return result
	})
}

func buildSemanticRouteResults(
	db *sql.DB,
	routeByID map[string]Route,
	scored []semanticScoredRoute,
	descriptor SemanticEmbedderDescriptor,
) ([]SpecSearchResult, error) {
	results := make([]SpecSearchResult, 0, len(scored)*3)

	for _, candidate := range scored {
		route, ok := routeByID[candidate.RouteID]
		if !ok {
			continue
		}

		candidateResults, err := buildSemanticRouteResultsForRoute(db, route, func(index int, result SpecSearchResult) SpecSearchResult {
			result.Rank = -500 - candidate.Score + float64(index)
			result.Tier = SpecSearchTierSemantic
			result.Reason = formatSemanticRouteReason(route.Title, descriptor, candidate.Score)
			return result
		})
		if err != nil {
			return nil, err
		}
		results = append(results, candidateResults...)
	}

	return results, nil
}

func buildSemanticRouteResultsForRoute(
	db *sql.DB,
	route Route,
	transform func(index int, result SpecSearchResult) SpecSearchResult,
) ([]SpecSearchResult, error) {
	patternIDs := append([]string{}, route.Core...)
	if len(patternIDs) == 0 {
		patternIDs = append(patternIDs, route.Chain...)
	}

	results := make([]SpecSearchResult, 0, len(patternIDs))
	for index, patternID := range patternIDs {
		result, err := searchByPatternID(db, patternID)
		if err != nil {
			continue
		}
		results = append(results, transform(index, result))
	}

	return results, nil
}

func buildSemanticDocumentResults(
	db *sql.DB,
	scored []semanticScoredDocument,
	descriptor SemanticEmbedderDescriptor,
) []SpecSearchResult {
	results := make([]SpecSearchResult, 0, len(scored))

	for _, candidate := range scored {
		result, err := searchByPatternID(db, candidate.PatternID)
		if err != nil {
			continue
		}
		result.Rank = -candidate.Score
		result.Tier = SpecSearchTierSemantic
		result.Reason = formatSemanticReason(descriptor, candidate.Score)
		results = append(results, result)
	}

	return results
}

func scoreSemanticArtifactDocuments(
	documents []SemanticArtifactDocument,
	queryVector []float32,
	minScore float64,
) []semanticScoredDocument {
	scored := make([]semanticScoredDocument, 0, len(documents))

	for _, document := range documents {
		score := cosineForNormalizedEmbeddings(document.Vector, queryVector)
		if score <= minScore {
			continue
		}
		scored = append(scored, semanticScoredDocument{
			PatternID: document.PatternID,
			Score:     score,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].PatternID < scored[j].PatternID
		}
		return scored[i].Score > scored[j].Score
	})

	return scored
}

func scoreSemanticArtifactRoutes(
	routes []SemanticArtifactRoute,
	queryVector []float32,
	minScore float64,
) []semanticScoredRoute {
	scored := make([]semanticScoredRoute, 0, len(routes))

	for _, route := range routes {
		score := cosineForNormalizedEmbeddings(route.Vector, queryVector)
		if score <= minScore {
			continue
		}
		scored = append(scored, semanticScoredRoute{
			RouteID: route.RouteID,
			Score:   score,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].RouteID < scored[j].RouteID
		}
		return scored[i].Score > scored[j].Score
	})

	return scored
}

func filterSemanticDocuments(
	scored []semanticScoredDocument,
	seen map[string]struct{},
) []semanticScoredDocument {
	filtered := make([]semanticScoredDocument, 0, len(scored))
	for _, candidate := range scored {
		if _, ok := seen[candidate.PatternID]; ok {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func truncateSemanticDocuments(scored []semanticScoredDocument, limit int) []semanticScoredDocument {
	if limit <= 0 || len(scored) <= limit {
		return scored
	}
	return scored[:limit]
}

func truncateSemanticRoutes(scored []semanticScoredRoute, limit int) []semanticScoredRoute {
	if limit <= 0 || len(scored) <= limit {
		return scored
	}
	return scored[:limit]
}

func sortedSemanticResults(resultsByKey map[string]SpecSearchResult, limit int) []SpecSearchResult {
	results := make([]SpecSearchResult, 0, len(resultsByKey))
	for _, result := range resultsByKey {
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Rank == results[j].Rank {
			if results[i].PatternID == results[j].PatternID {
				return results[i].Heading < results[j].Heading
			}
			return results[i].PatternID < results[j].PatternID
		}
		return results[i].Rank < results[j].Rank
	})

	if len(results) <= limit {
		return results
	}
	return results[:limit]
}

func resultPatternSet(resultsByKey map[string]SpecSearchResult) map[string]struct{} {
	patterns := make(map[string]struct{}, len(resultsByKey))
	for _, result := range resultsByKey {
		patterns[result.PatternID] = struct{}{}
	}
	return patterns
}

func formatSemanticReason(
	descriptor SemanticEmbedderDescriptor,
	score float64,
) string {
	return fmt.Sprintf(
		"embedding cosine %.3f via %s/%s",
		score,
		descriptor.Provider,
		descriptor.Model,
	)
}

func formatSemanticRouteReason(
	title string,
	descriptor SemanticEmbedderDescriptor,
	score float64,
) string {
	return fmt.Sprintf(
		"embedding route %s %.3f via %s/%s",
		title,
		score,
		descriptor.Provider,
		descriptor.Model,
	)
}

func normalizeEmbeddingVector(vector []float32) []float32 {
	norm := embeddingVectorNorm(vector)
	if norm == 0 {
		return nil
	}

	normalized := make([]float32, 0, len(vector))
	for _, value := range vector {
		normalized = append(normalized, value/float32(norm))
	}
	return normalized
}

func embeddingVectorNorm(vector []float32) float64 {
	squared := 0.0
	for _, value := range vector {
		squared += float64(value * value)
	}
	return math.Sqrt(squared)
}

func cosineForNormalizedEmbeddings(left []float32, right []float32) float64 {
	if len(left) == 0 || len(right) == 0 || len(left) != len(right) {
		return 0
	}

	score := 0.0
	for index := range left {
		score += float64(left[index] * right[index])
	}
	return score
}
