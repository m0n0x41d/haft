package fpf

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	SpecSearchTierSemantic           = "semantic"
	defaultSemanticBodyPreviewLimit  = 1200
	defaultSemanticSnippetLimit      = 280
	defaultSemanticMinimumSimilarity = 0
	maxSemanticRouteCandidates       = 1
)

// SemanticSearchOptions controls the experimental vector-style retrieval path.
type SemanticSearchOptions struct {
	Limit    int
	MinScore float64
}

type semanticVector map[string]float64

type semanticDocument struct {
	PatternID   string
	Heading     string
	Summary     string
	Snippet     string
	BodyPreview string
	Keywords    []string
	Queries     []string
	Aliases     []string
	Vector      semanticVector
	Norm        float64
}

type semanticScoredDocument struct {
	Document semanticDocument
	Score    float64
}

type semanticRouteDocument struct {
	Route  Route
	Vector semanticVector
	Norm   float64
}

type semanticScoredRoute struct {
	Route Route
	Score float64
}

type semanticCorpus struct {
	documents []semanticDocument
	idf       semanticVector
}

type semanticRouteCorpus struct {
	routes []semanticRouteDocument
	idf    semanticVector
}

type semanticQueryVector struct {
	weights semanticVector
	norm    float64
}

var semanticStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"do": {}, "for": {}, "from": {}, "how": {}, "i": {}, "if": {}, "in": {}, "into": {},
	"is": {}, "it": {}, "of": {}, "on": {}, "or": {}, "so": {}, "that": {}, "the": {},
	"this": {}, "to": {}, "up": {}, "we": {}, "what": {}, "when": {}, "with": {}, "without": {},
}

// SearchSpecSemantically runs the experimental local vector-search prototype.
//
// This path is intentionally separate from SearchSpecWithOptions so the
// deterministic route-aware retrieval stack stays untouched by default.
func SearchSpecSemantically(db *sql.DB, query string, options SemanticSearchOptions) ([]SpecSearchResult, error) {
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

	routeResults, err := searchSemanticRoutes(db, query, options)
	if err != nil {
		return nil, err
	}
	appendResults(routeResults)

	documents, err := loadSemanticDocuments(db)
	if err != nil {
		return nil, err
	}

	corpus := buildSemanticCorpus(documents)
	queryVector := buildSemanticQueryVector(query, corpus.idf)
	if queryVector.norm == 0 {
		return sortedSemanticResults(resultsByKey, options.Limit), nil
	}

	scored := scoreSemanticDocuments(corpus.documents, queryVector, options.MinScore)
	scored = filterSemanticDocuments(scored, resultPatternSet(resultsByKey))
	scored = truncateSemanticDocuments(scored, options.Limit-len(resultsByKey))
	appendResults(buildSemanticResults(scored))

	return sortedSemanticResults(resultsByKey, options.Limit), nil
}

func normalizeSemanticSearchOptions(options SemanticSearchOptions) SemanticSearchOptions {
	if options.Limit <= 0 {
		options.Limit = DefaultSpecSearchLimit
	}
	if options.MinScore <= 0 {
		options.MinScore = defaultSemanticMinimumSimilarity
	}
	return options
}

func semanticPatternOnlyQuery(query string) bool {
	normalized := normalizePatternID(query)
	return normalized != "" && normalized == extractPatternID(query)
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

func searchSemanticRoutes(db *sql.DB, query string, options SemanticSearchOptions) ([]SpecSearchResult, error) {
	route, err := classifyRoute(db, query)
	if err != nil {
		return nil, err
	}
	if route != nil {
		return buildSemanticSeedRouteResults(db, *route)
	}

	routes, err := loadSemanticRoutes(db)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, nil
	}

	corpus := buildSemanticRouteCorpus(routes)
	queryVector := buildSemanticQueryVector(query, corpus.idf)
	if queryVector.norm == 0 {
		return nil, nil
	}

	scored := scoreSemanticRoutes(corpus.routes, queryVector, options.MinScore)
	scored = truncateSemanticRoutes(scored, maxSemanticRouteCandidates)
	return buildSemanticRouteResults(db, scored)
}

func loadSemanticRoutes(db *sql.DB) ([]Route, error) {
	rows, err := db.Query(`SELECT route_id, title, description, matchers_json, core_json, chain_json FROM routes ORDER BY route_id`)
	if err != nil {
		return nil, fmt.Errorf("load semantic routes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	routes := make([]Route, 0, 32)
	for rows.Next() {
		route := Route{}
		matchersJSON := ""
		coreJSON := ""
		chainJSON := ""

		err := rows.Scan(
			&route.ID,
			&route.Title,
			&route.Description,
			&matchersJSON,
			&coreJSON,
			&chainJSON,
		)
		if err != nil {
			return nil, err
		}

		route.Matchers = decodeSemanticStringList(matchersJSON)
		route.Core = decodeSemanticStringList(coreJSON)
		route.Chain = decodeSemanticStringList(chainJSON)
		routes = append(routes, route)
	}

	return routes, rows.Err()
}

func loadSemanticDocuments(db *sql.DB) ([]semanticDocument, error) {
	rows, err := db.Query(`
		SELECT
			pattern_id,
			heading,
			summary,
			substr(body, 1, ?),
			substr(body, 1, ?),
			keywords_json,
			queries_json,
			aliases_json
		FROM sections
		WHERE pattern_id IS NOT NULL
		ORDER BY pattern_id, heading
	`, defaultSemanticSnippetLimit, defaultSemanticBodyPreviewLimit)
	if err != nil {
		return nil, fmt.Errorf("load semantic documents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	documents := make([]semanticDocument, 0, 256)
	for rows.Next() {
		document, err := scanSemanticDocument(rows)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}

	return documents, rows.Err()
}

func scanSemanticDocument(scanner interface {
	Scan(dest ...any) error
}) (semanticDocument, error) {
	document := semanticDocument{}
	keywordsJSON := ""
	queriesJSON := ""
	aliasesJSON := ""

	err := scanner.Scan(
		&document.PatternID,
		&document.Heading,
		&document.Summary,
		&document.Snippet,
		&document.BodyPreview,
		&keywordsJSON,
		&queriesJSON,
		&aliasesJSON,
	)
	if err != nil {
		return semanticDocument{}, err
	}

	document.Keywords = decodeSemanticStringList(keywordsJSON)
	document.Queries = decodeSemanticStringList(queriesJSON)
	document.Aliases = decodeSemanticStringList(aliasesJSON)
	document.Summary = firstNonEmpty(document.Summary, fallbackSectionSummary(document.Heading))
	document.Snippet = firstNonEmpty(document.Snippet, document.Summary)
	document.BodyPreview = firstNonEmpty(document.BodyPreview, document.Snippet)

	return document, nil
}

func decodeSemanticStringList(raw string) []string {
	items := []string{}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

func buildSemanticCorpus(documents []semanticDocument) semanticCorpus {
	rawVectors := make([]semanticVector, 0, len(documents))
	documentFrequency := make(map[string]int)

	for _, document := range documents {
		rawVector := buildSemanticDocumentVector(document)
		rawVectors = append(rawVectors, rawVector)
		documentFrequency = incrementSemanticDocumentFrequency(documentFrequency, rawVector)
	}

	idf := buildSemanticIDF(len(documents), documentFrequency)
	weightedDocuments := make([]semanticDocument, 0, len(documents))

	for index, document := range documents {
		vector := applySemanticIDF(rawVectors[index], idf)
		document.Vector = vector
		document.Norm = semanticVectorNorm(vector)
		weightedDocuments = append(weightedDocuments, document)
	}

	return semanticCorpus{
		documents: weightedDocuments,
		idf:       idf,
	}
}

func buildSemanticRouteCorpus(routes []Route) semanticRouteCorpus {
	rawVectors := make([]semanticVector, 0, len(routes))
	documentFrequency := make(map[string]int)

	for _, route := range routes {
		rawVector := buildSemanticRouteVector(route)
		rawVectors = append(rawVectors, rawVector)
		documentFrequency = incrementSemanticDocumentFrequency(documentFrequency, rawVector)
	}

	idf := buildSemanticIDF(len(routes), documentFrequency)
	weightedRoutes := make([]semanticRouteDocument, 0, len(routes))

	for index, route := range routes {
		vector := applySemanticIDF(rawVectors[index], idf)
		weightedRoutes = append(weightedRoutes, semanticRouteDocument{
			Route:  route,
			Vector: vector,
			Norm:   semanticVectorNorm(vector),
		})
	}

	return semanticRouteCorpus{
		routes: weightedRoutes,
		idf:    idf,
	}
}

func buildSemanticDocumentVector(document semanticDocument) semanticVector {
	vector := make(semanticVector)

	vector = appendSemanticTextVector(vector, document.Heading, 5.0)
	vector = appendSemanticTextVector(vector, document.Summary, 4.0)
	vector = appendSemanticTextVector(vector, document.BodyPreview, 1.25)

	for _, alias := range document.Aliases {
		vector = appendSemanticTextVector(vector, alias, 4.5)
	}
	for _, keyword := range document.Keywords {
		vector = appendSemanticTextVector(vector, keyword, 3.5)
	}
	for _, query := range document.Queries {
		vector = appendSemanticTextVector(vector, query, 4.0)
	}

	return vector
}

func buildSemanticRouteVector(route Route) semanticVector {
	vector := make(semanticVector)

	vector = appendSemanticTextVector(vector, route.Title, 5.0)
	vector = appendSemanticTextVector(vector, route.Description, 4.0)

	for _, matcher := range route.Matchers {
		vector = appendSemanticTextVector(vector, matcher, 5.0)
	}

	return vector
}

func incrementSemanticDocumentFrequency(documentFrequency map[string]int, vector semanticVector) map[string]int {
	for feature := range vector {
		documentFrequency[feature]++
	}
	return documentFrequency
}

func buildSemanticIDF(totalDocuments int, documentFrequency map[string]int) semanticVector {
	idf := make(semanticVector, len(documentFrequency))
	documentCount := float64(totalDocuments)

	for feature, frequency := range documentFrequency {
		frequencyCount := float64(frequency)
		idf[feature] = 1 + math.Log((1+documentCount)/(1+frequencyCount))
	}

	return idf
}

func applySemanticIDF(rawVector semanticVector, idf semanticVector) semanticVector {
	weighted := make(semanticVector, len(rawVector))

	for feature, rawWeight := range rawVector {
		if rawWeight <= 0 {
			continue
		}

		tfWeight := 1 + math.Log(rawWeight)
		weighted[feature] = tfWeight * idf[feature]
	}

	return weighted
}

func buildSemanticQueryVector(query string, idf semanticVector) semanticQueryVector {
	rawVector := make(semanticVector)
	rawVector = appendSemanticTextVector(rawVector, query, 1.0)

	weighted := applySemanticIDF(rawVector, idf)
	norm := semanticVectorNorm(weighted)

	return semanticQueryVector{
		weights: weighted,
		norm:    norm,
	}
}

func appendSemanticTextVector(vector semanticVector, text string, weight float64) semanticVector {
	tokens := semanticTokens(text)
	if len(tokens) == 0 {
		return vector
	}

	for _, token := range tokens {
		vector["tok:"+token] += weight
	}

	for _, bigram := range semanticBigrams(tokens) {
		vector["bigram:"+bigram] += weight * 0.8
	}

	for _, trigram := range semanticTokenTrigrams(tokens) {
		vector["tri:"+trigram] += weight * 0.2
	}

	return vector
}

func semanticTokens(text string) []string {
	text = strings.ToLower(cleanMarkdownText(text))
	if text == "" {
		return nil
	}

	tokens := make([]string, 0, 32)
	current := strings.Builder{}

	flush := func() {
		token := current.String()
		current.Reset()
		if !keepSemanticToken(token) {
			return
		}
		tokens = append(tokens, token)
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}

	flush()
	return tokens
}

func keepSemanticToken(token string) bool {
	if len(token) < 2 {
		return false
	}
	if _, ok := semanticStopWords[token]; ok {
		return false
	}
	return true
}

func semanticBigrams(tokens []string) []string {
	if len(tokens) < 2 {
		return nil
	}

	bigrams := make([]string, 0, len(tokens)-1)
	for index := 0; index < len(tokens)-1; index++ {
		bigrams = append(bigrams, tokens[index]+" "+tokens[index+1])
	}

	return bigrams
}

func semanticTokenTrigrams(tokens []string) []string {
	trigrams := make([]string, 0, len(tokens)*3)

	for _, token := range tokens {
		trigrams = append(trigrams, tokenTrigrams(token)...)
	}

	return trigrams
}

func tokenTrigrams(token string) []string {
	runes := []rune(token)
	if len(runes) < 4 {
		return nil
	}

	padded := append([]rune{'^'}, runes...)
	padded = append(padded, '$')
	trigrams := make([]string, 0, len(padded)-2)

	for index := 0; index <= len(padded)-3; index++ {
		trigrams = append(trigrams, string(padded[index:index+3]))
	}

	return trigrams
}

func scoreSemanticDocuments(documents []semanticDocument, query semanticQueryVector, minScore float64) []semanticScoredDocument {
	scored := make([]semanticScoredDocument, 0, len(documents))

	for _, document := range documents {
		if document.Norm == 0 {
			continue
		}

		score := semanticCosineSimilarity(document.Vector, document.Norm, query.weights, query.norm)
		if score < minScore {
			continue
		}

		scored = append(scored, semanticScoredDocument{
			Document: document,
			Score:    score,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Document.PatternID < scored[j].Document.PatternID
		}
		return scored[i].Score > scored[j].Score
	})

	return scored
}

func scoreSemanticRoutes(routes []semanticRouteDocument, query semanticQueryVector, minScore float64) []semanticScoredRoute {
	scored := make([]semanticScoredRoute, 0, len(routes))

	for _, route := range routes {
		if route.Norm == 0 {
			continue
		}

		score := semanticCosineSimilarity(route.Vector, route.Norm, query.weights, query.norm)
		if score < minScore {
			continue
		}

		scored = append(scored, semanticScoredRoute{
			Route: route.Route,
			Score: score,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Route.ID < scored[j].Route.ID
		}
		return scored[i].Score > scored[j].Score
	})

	return scored
}

func semanticCosineSimilarity(documentVector semanticVector, documentNorm float64, queryVector semanticVector, queryNorm float64) float64 {
	if documentNorm == 0 || queryNorm == 0 {
		return 0
	}

	dotProduct := 0.0
	for feature, queryWeight := range queryVector {
		dotProduct += queryWeight * documentVector[feature]
	}

	return dotProduct / (documentNorm * queryNorm)
}

func semanticVectorNorm(vector semanticVector) float64 {
	squared := 0.0
	for _, weight := range vector {
		squared += weight * weight
	}
	return math.Sqrt(squared)
}

func filterSemanticDocuments(scored []semanticScoredDocument, seen map[string]struct{}) []semanticScoredDocument {
	filtered := make([]semanticScoredDocument, 0, len(scored))

	for _, candidate := range scored {
		if _, ok := seen[candidate.Document.PatternID]; ok {
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

func buildSemanticRouteResults(db *sql.DB, scored []semanticScoredRoute) ([]SpecSearchResult, error) {
	results := make([]SpecSearchResult, 0, len(scored)*3)

	for _, candidate := range scored {
		candidateResults, err := buildSemanticRouteResultsForRoute(db, candidate.Route, func(index int, result SpecSearchResult) SpecSearchResult {
			result.Rank = -500 - candidate.Score + float64(index)
			result.Tier = SpecSearchTierSemantic
			result.Reason = formatSemanticRouteReason(candidate.Route.Title, candidate.Score)
			return result
		})
		if err != nil {
			return nil, err
		}
		results = append(results, candidateResults...)
	}

	return results, nil
}

func buildSemanticResults(scored []semanticScoredDocument) []SpecSearchResult {
	results := make([]SpecSearchResult, 0, len(scored))

	for _, candidate := range scored {
		results = append(results, SpecSearchResult{
			PatternID: candidate.Document.PatternID,
			Heading:   candidate.Document.Heading,
			Summary:   candidate.Document.Summary,
			Snippet:   candidate.Document.Snippet,
			Rank:      -candidate.Score,
			Tier:      SpecSearchTierSemantic,
			Reason:    formatSemanticReason(candidate.Score),
		})
	}

	return results
}

func formatSemanticReason(score float64) string {
	return fmt.Sprintf("experimental tf-idf similarity %.3f", score)
}

func formatSemanticRouteReason(title string, score float64) string {
	return fmt.Sprintf("semantic route %s %.3f", title, score)
}

func buildSemanticSeedRouteResults(db *sql.DB, route Route) ([]SpecSearchResult, error) {
	return buildSemanticRouteResultsForRoute(db, route, func(index int, result SpecSearchResult) SpecSearchResult {
		result.Rank = -600 + float64(index)
		result.Tier = SpecSearchTierSemantic
		result.Reason = "semantic seed " + route.Title
		return result
	})
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
