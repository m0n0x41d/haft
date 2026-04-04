package fpf

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// SpecSearchResult represents a single FPF spec search hit.
type SpecSearchResult struct {
	PatternID string
	Heading   string
	Summary   string
	Snippet   string
	Rank      float64
	Tier      string
	Reason    string
}

const (
	SpecSearchTierPattern = "pattern"
	SpecSearchTierRoute   = "route"
	SpecSearchTierRelated = "related"
	SpecSearchTierFTS     = "fts"
)

const DefaultSpecSearchLimit = 10

// SpecSearchOptions controls deterministic retrieval behavior for FPF queries.
type SpecSearchOptions struct {
	Limit int
	Tier  string
}

type Route struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Matchers    []string `json:"matchers"`
	Core        []string `json:"core"`
	Chain       []string `json:"chain"`
}

// SpecIndexSchemaVersion identifies the current SQLite index layout contract.
const SpecIndexSchemaVersion = "2"

// SpecIndexInfo exposes inspectable build provenance for the embedded index.
type SpecIndexInfo struct {
	Commit          string
	IndexedSections string
	BuildTime       string
	SpecPath        string
	SchemaVersion   string
}

const relatedExpansionLimit = 10

var relatedExpansionPreferredEdgeTypes = []SpecEdgeType{
	SpecEdgeTypeBuildsOn,
	SpecEdgeTypePrerequisiteFor,
	SpecEdgeTypeCoordinatesWith,
}

var relatedExpansionFallbackEdgeTypes = []SpecEdgeType{
	SpecEdgeTypeConstrains,
	SpecEdgeTypeInforms,
	SpecEdgeTypeUsedBy,
	SpecEdgeTypeRefines,
	SpecEdgeTypeSpecialisedBy,
	SpecEdgeTypeRelated,
}

type relatedExpansionPolicy struct {
	MaxResults int
}

type relatedCandidate struct {
	FromPatternID string
	ToPatternID   string
	Heading       string
	Summary       string
	Snippet       string
	EdgeType      SpecEdgeType
}

var defaultRelatedExpansionPolicy = relatedExpansionPolicy{
	MaxResults: relatedExpansionLimit,
}

// BuildSpecIndex creates a structured SQLite index from spec chunks and route artifacts.
func BuildSpecIndex(dbPath string, chunks []SpecChunk, routes []Route) error {
	normalizedChunks := normalizeChunksForIndex(chunks)
	normalizedRoutes := normalizeRoutesForIndex(routes)

	if err := validateRouteShape(normalizedRoutes); err != nil {
		return fmt.Errorf("validate routes: %w", err)
	}
	if err := validateRouteReferences(normalizedRoutes, normalizedChunks); err != nil {
		return fmt.Errorf("validate routes: %w", err)
	}

	_ = os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE sections (
			id INTEGER PRIMARY KEY,
			pattern_id TEXT,
			heading TEXT NOT NULL,
			level INTEGER NOT NULL,
			body TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			parent_pattern_id TEXT,
			keywords_json TEXT NOT NULL DEFAULT '[]',
			queries_json TEXT NOT NULL DEFAULT '[]',
			aliases_json TEXT NOT NULL DEFAULT '[]'
		)`,
		`CREATE VIRTUAL TABLE fpf_fts USING fts5(pattern_id, heading, body, keywords, queries, aliases, tokenize='porter unicode61')`,
		`CREATE TABLE section_edges (
			from_pattern_id TEXT NOT NULL,
			to_pattern_id TEXT NOT NULL,
			edge_type TEXT NOT NULL
		)`,
		`CREATE TABLE routes (
			route_id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			matchers_json TEXT NOT NULL,
			core_json TEXT NOT NULL,
			chain_json TEXT NOT NULL
		)`,
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s, err)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	secIns, err := tx.Prepare(`INSERT INTO sections (id, pattern_id, heading, level, body, summary, parent_pattern_id, keywords_json, queries_json, aliases_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = secIns.Close() }()

	ftsIns, err := tx.Prepare(`INSERT INTO fpf_fts (pattern_id, heading, body, keywords, queries, aliases) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = ftsIns.Close() }()

	edgeIns, err := tx.Prepare(`INSERT INTO section_edges (from_pattern_id, to_pattern_id, edge_type) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = edgeIns.Close() }()

	routeIns, err := tx.Prepare(`INSERT INTO routes (route_id, title, description, matchers_json, core_json, chain_json) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = routeIns.Close() }()

	seenEdges := make(map[string]struct{})
	for _, c := range normalizedChunks {
		keywordsJSON := mustJSON(c.Keywords)
		queriesJSON := mustJSON(c.Queries)
		aliasesJSON := mustJSON(c.Aliases)
		if _, err := secIns.Exec(c.ID, nullIfEmpty(c.PatternID), c.Heading, c.Level, c.Body, c.Summary, nullIfEmpty(c.ParentPatternID), keywordsJSON, queriesJSON, aliasesJSON); err != nil {
			return fmt.Errorf("insert section %d: %w", c.ID, err)
		}
		if _, err := ftsIns.Exec(c.PatternID, c.Heading, c.Body, strings.Join(c.Keywords, " "), strings.Join(c.Queries, " "), strings.Join(c.Aliases, " ")); err != nil {
			return fmt.Errorf("insert fts section %d: %w", c.ID, err)
		}
		for _, edge := range c.Edges {
			if edge.FromPatternID == "" || edge.ToPatternID == "" || edge.FromPatternID == edge.ToPatternID {
				continue
			}
			key := edgeKey(edge)
			if _, ok := seenEdges[key]; ok {
				continue
			}
			seenEdges[key] = struct{}{}
			if _, err := edgeIns.Exec(edge.FromPatternID, edge.ToPatternID, edge.EdgeType); err != nil {
				return fmt.Errorf("insert edge %s: %w", key, err)
			}
		}
		for _, related := range c.RelatedIDs {
			if c.PatternID == "" || related == "" || related == c.PatternID {
				continue
			}
			edge := SpecEdge{
				FromPatternID: c.PatternID,
				ToPatternID:   related,
				EdgeType:      SpecEdgeTypeRelated,
			}
			key := edgeKey(edge)
			if _, ok := seenEdges[key]; ok {
				continue
			}
			seenEdges[key] = struct{}{}
			if _, err := edgeIns.Exec(edge.FromPatternID, edge.ToPatternID, edge.EdgeType); err != nil {
				return fmt.Errorf("insert edge %s: %w", key, err)
			}
		}
	}

	for _, route := range normalizedRoutes {
		if _, err := routeIns.Exec(route.ID, route.Title, route.Description, mustJSON(route.Matchers), mustJSON(route.Core), mustJSON(route.Chain)); err != nil {
			return fmt.Errorf("insert route %s: %w", route.ID, err)
		}
	}

	return tx.Commit()
}

// SetSpecMeta writes a key-value pair to the meta table.
func SetSpecMeta(dbPath, key, value string) error {
	return SetSpecMetaEntries(dbPath, map[string]string{
		key: value,
	})
}

// SetSpecMetaEntries writes metadata entries to the meta table in one transaction.
func SetSpecMetaEntries(dbPath string, entries map[string]string) error {
	if len(entries) == 0 {
		return nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer func() { _ = stmt.Close() }()

	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, err := stmt.Exec(key, entries[key]); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// SearchSpec queries the structured index with exact-id, route, related, and FTS fallback tiers.
func SearchSpec(db *sql.DB, query string, limit int) ([]SpecSearchResult, error) {
	return SearchSpecWithOptions(db, query, SpecSearchOptions{
		Limit: limit,
	})
}

// SearchSpecWithOptions queries the structured index with deterministic search controls.
func SearchSpecWithOptions(db *sql.DB, query string, options SpecSearchOptions) ([]SpecSearchResult, error) {
	if options.Limit <= 0 {
		options.Limit = DefaultSpecSearchLimit
	}

	normalizedTier, err := NormalizeSpecSearchTier(options.Tier)
	if err != nil {
		return nil, err
	}
	options.Tier = normalizedTier

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	resultsMap := make(map[string]SpecSearchResult)
	appendResults := func(results []SpecSearchResult) {
		for _, result := range results {
			key := result.PatternID + "|" + result.Heading
			if existing, ok := resultsMap[key]; ok {
				if result.Rank < existing.Rank {
					resultsMap[key] = result
				}
				continue
			}
			resultsMap[key] = result
		}
	}

	if shouldIncludeSpecSearchTier(options.Tier, SpecSearchTierPattern) {
		patternID := extractPatternID(query)
		if patternID != "" {
			if result, err := searchByPatternID(db, patternID); err == nil {
				appendResults([]SpecSearchResult{result})
			}
		}
	}

	route, err := classifyRoute(db, query)
	if err != nil {
		return nil, err
	}
	if route != nil && shouldIncludeSpecSearchTier(options.Tier, SpecSearchTierRoute) {
		routeResults, err := searchRoute(db, *route)
		if err != nil {
			return nil, err
		}
		appendResults(routeResults)
	}
	if route != nil && shouldIncludeSpecSearchTier(options.Tier, SpecSearchTierRelated) {
		relatedLimit := effectiveRelatedExpansionLimit(options)

		edgeResults, err := searchRelated(db, route.Core, relatedLimit)
		if err != nil {
			return nil, err
		}
		appendResults(edgeResults)
	}

	if shouldIncludeSpecSearchTier(options.Tier, SpecSearchTierFTS) {
		ftsResults, err := searchFTS(db, query, options.Limit*2)
		if err != nil {
			return nil, err
		}
		appendResults(ftsResults)
	}

	results := make([]SpecSearchResult, 0, len(resultsMap))
	for _, result := range resultsMap {
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
	if len(results) > options.Limit {
		results = results[:options.Limit]
	}
	return results, nil
}

func effectiveRelatedExpansionLimit(options SpecSearchOptions) int {
	if options.Tier != SpecSearchTierRelated {
		return defaultRelatedExpansionPolicy.MaxResults
	}
	return clampRelatedExpansionLimit(options.Limit)
}

func clampRelatedExpansionLimit(limit int) int {
	if limit <= 0 {
		return relatedExpansionLimit
	}
	if limit > relatedExpansionLimit {
		return relatedExpansionLimit
	}
	return limit
}

// NormalizeSpecSearchTier validates and canonicalizes a tier filter.
func NormalizeSpecSearchTier(tier string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(tier))
	switch normalized {
	case "", SpecSearchTierPattern, SpecSearchTierRoute, SpecSearchTierRelated, SpecSearchTierFTS:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported search tier %q (want pattern, route, related, or fts)", tier)
	}
}

func shouldIncludeSpecSearchTier(filter string, tier string) bool {
	return filter == "" || filter == tier
}

func searchByPatternID(db *sql.DB, patternID string) (SpecSearchResult, error) {
	patternID = normalizePatternID(patternID)

	var result SpecSearchResult
	err := db.QueryRow(`
		SELECT pattern_id, heading, summary, substr(body, 1, 280)
		FROM sections
		WHERE pattern_id = ?
	`, patternID).Scan(&result.PatternID, &result.Heading, &result.Summary, &result.Snippet)
	if err != nil {
		return SpecSearchResult{}, err
	}
	result.Rank = -1000
	result.Tier = SpecSearchTierPattern
	result.Reason = "exact pattern id"
	return result, nil
}

func searchRoute(db *sql.DB, route Route) ([]SpecSearchResult, error) {
	patternIDs := append([]string{}, route.Core...)
	if len(patternIDs) == 0 {
		patternIDs = append(patternIDs, route.Chain...)
	}
	results := make([]SpecSearchResult, 0, len(patternIDs))
	for i, patternID := range patternIDs {
		if result, err := searchByPatternID(db, patternID); err == nil {
			result.Rank = -500 + float64(i)
			result.Tier = SpecSearchTierRoute
			result.Reason = route.Title
			results = append(results, result)
		}
	}
	return results, nil
}

func searchRelated(db *sql.DB, seeds []string, maxResults int) ([]SpecSearchResult, error) {
	if len(seeds) == 0 {
		return nil, nil
	}

	candidates, err := loadRelatedCandidates(db, seeds)
	if err != nil {
		return nil, err
	}

	policy := defaultRelatedExpansionPolicy
	policy.MaxResults = clampRelatedExpansionLimit(maxResults)

	selected := selectRelatedCandidates(candidates, seeds, policy)
	results := buildRelatedResults(selected)
	return results, nil
}

func loadRelatedCandidates(db *sql.DB, seeds []string) ([]relatedCandidate, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(seeds)), ",")
	args := make([]any, 0, len(seeds))
	for _, seed := range seeds {
		args = append(args, seed)
	}
	rows, err := db.Query(fmt.Sprintf(`
		SELECT e.from_pattern_id, e.to_pattern_id, e.edge_type, s.heading, s.summary, substr(s.body, 1, 280)
		FROM section_edges e
		JOIN sections s ON s.pattern_id = e.to_pattern_id
		WHERE e.from_pattern_id IN (%s)
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var candidates []relatedCandidate
	for rows.Next() {
		var candidate relatedCandidate
		var edgeType string
		if err := rows.Scan(&candidate.FromPatternID, &candidate.ToPatternID, &edgeType, &candidate.Heading, &candidate.Summary, &candidate.Snippet); err != nil {
			return nil, err
		}
		candidate.EdgeType = SpecEdgeType(edgeType)
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func selectRelatedCandidates(candidates []relatedCandidate, seeds []string, policy relatedExpansionPolicy) []relatedCandidate {
	if len(candidates) == 0 {
		return nil
	}

	if policy.MaxResults <= 0 {
		policy.MaxResults = relatedExpansionLimit
	}

	seedOrder := buildSeedOrder(seeds)
	deduped := dedupeRelatedCandidates(candidates, seedOrder)
	preferred := filterRelatedCandidates(deduped, isPreferredRelatedEdge)
	fallback := filterRelatedCandidates(deduped, func(candidate relatedCandidate) bool {
		return !isPreferredRelatedEdge(candidate)
	})

	sortRelatedCandidates(preferred, seedOrder, relatedExpansionPreferredEdgeTypes)
	sortRelatedCandidates(fallback, seedOrder, relatedExpansionFallbackEdgeTypes)

	selected := appendRelatedCandidates(preferred, fallback)
	selected = truncateRelatedCandidates(selected, policy.MaxResults)
	return selected
}

func buildRelatedResults(candidates []relatedCandidate) []SpecSearchResult {
	results := make([]SpecSearchResult, 0, len(candidates))
	for index, candidate := range candidates {
		result := SpecSearchResult{
			PatternID: candidate.ToPatternID,
			Heading:   candidate.Heading,
			Summary:   candidate.Summary,
			Snippet:   candidate.Snippet,
			Rank:      -200 + float64(index),
			Tier:      SpecSearchTierRelated,
			Reason:    formatRelatedReason(candidate),
		}
		results = append(results, result)
	}
	return results
}

func buildSeedOrder(seeds []string) map[string]int {
	order := make(map[string]int, len(seeds))
	for index, seed := range seeds {
		if _, ok := order[seed]; ok {
			continue
		}
		order[seed] = index
	}
	return order
}

func dedupeRelatedCandidates(candidates []relatedCandidate, seedOrder map[string]int) []relatedCandidate {
	deduped := make(map[string]relatedCandidate, len(candidates))
	for _, candidate := range candidates {
		existing, ok := deduped[candidate.ToPatternID]
		if ok && !relatedCandidateLess(candidate, existing, seedOrder, relatedExpansionPreferredEdgeTypes, relatedExpansionFallbackEdgeTypes) {
			continue
		}
		deduped[candidate.ToPatternID] = candidate
	}

	results := make([]relatedCandidate, 0, len(deduped))
	for _, candidate := range deduped {
		results = append(results, candidate)
	}
	return results
}

func filterRelatedCandidates(candidates []relatedCandidate, keep func(relatedCandidate) bool) []relatedCandidate {
	filtered := make([]relatedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if keep(candidate) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func isPreferredRelatedEdge(candidate relatedCandidate) bool {
	return edgeOrder(candidate.EdgeType, relatedExpansionPreferredEdgeTypes) >= 0
}

func sortRelatedCandidates(candidates []relatedCandidate, seedOrder map[string]int, edgeTypes []SpecEdgeType) {
	sort.Slice(candidates, func(i, j int) bool {
		return relatedCandidateLess(candidates[i], candidates[j], seedOrder, edgeTypes)
	})
}

func relatedCandidateLess(left, right relatedCandidate, seedOrder map[string]int, edgeOrders ...[]SpecEdgeType) bool {
	leftPriority := candidatePriority(left, edgeOrders...)
	rightPriority := candidatePriority(right, edgeOrders...)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}

	leftSeedOrder := seedIndex(left.FromPatternID, seedOrder)
	rightSeedOrder := seedIndex(right.FromPatternID, seedOrder)
	if leftSeedOrder != rightSeedOrder {
		return leftSeedOrder < rightSeedOrder
	}

	if left.ToPatternID != right.ToPatternID {
		return left.ToPatternID < right.ToPatternID
	}

	if left.Heading != right.Heading {
		return left.Heading < right.Heading
	}

	return left.FromPatternID < right.FromPatternID
}

func candidatePriority(candidate relatedCandidate, edgeOrders ...[]SpecEdgeType) int {
	offset := 0
	for _, edgeTypes := range edgeOrders {
		order := edgeOrder(candidate.EdgeType, edgeTypes)
		if order >= 0 {
			return offset + order
		}
		offset += len(edgeTypes)
	}
	return offset
}

func edgeOrder(edgeType SpecEdgeType, edgeTypes []SpecEdgeType) int {
	for index, candidate := range edgeTypes {
		if edgeType == candidate {
			return index
		}
	}
	return -1
}

func seedIndex(patternID string, order map[string]int) int {
	index, ok := order[patternID]
	if ok {
		return index
	}
	return len(order)
}

func truncateRelatedCandidates(candidates []relatedCandidate, maxResults int) []relatedCandidate {
	if len(candidates) <= maxResults {
		return candidates
	}
	return candidates[:maxResults]
}

func appendRelatedCandidates(groups ...[]relatedCandidate) []relatedCandidate {
	total := 0
	for _, group := range groups {
		total += len(group)
	}

	combined := make([]relatedCandidate, 0, total)
	for _, group := range groups {
		combined = append(combined, group...)
	}
	return combined
}

func formatRelatedReason(candidate relatedCandidate) string {
	return fmt.Sprintf("%s via %s", candidate.EdgeType, candidate.FromPatternID)
}

func searchFTS(db *sql.DB, query string, limit int) ([]SpecSearchResult, error) {
	terms := strings.Fields(query)
	var ftsTerms []string
	for _, t := range terms {
		t = strings.ReplaceAll(t, `"`, `""`)
		ftsTerms = append(ftsTerms, fmt.Sprintf(`"%s"*`, t))
	}
	ftsQuery := strings.Join(ftsTerms, " OR ")
	if len(terms) > 1 {
		var andTerms []string
		for _, t := range terms {
			quoted := strings.ReplaceAll(t, `"`, `""`)
			andTerms = append(andTerms, fmt.Sprintf(`"%s"*`, quoted))
		}
		andQuery := strings.Join(andTerms, " ")
		results, err := runFTSQuery(db, andQuery, limit)
		if err == nil && len(results) > 0 {
			return results, nil
		}
	}
	return runFTSQuery(db, ftsQuery, limit)
}

func runFTSQuery(db *sql.DB, ftsQuery string, limit int) ([]SpecSearchResult, error) {
	rows, err := db.Query(`
		SELECT s.pattern_id, s.heading, s.summary, snippet(fpf_fts, 2, '>>>', '<<<', '...', 64), rank
		FROM fpf_fts
		JOIN sections s ON s.id = fpf_fts.rowid - 1
		WHERE fpf_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []SpecSearchResult
	for rows.Next() {
		var r SpecSearchResult
		var patternID sql.NullString
		if err := rows.Scan(&patternID, &r.Heading, &r.Summary, &r.Snippet, &r.Rank); err != nil {
			return nil, err
		}
		r.PatternID = patternID.String
		r.Tier = SpecSearchTierFTS
		r.Reason = "keyword match"
		results = append(results, r)
	}
	return results, rows.Err()
}

func classifyRoute(db *sql.DB, query string) (*Route, error) {
	rows, err := db.Query(`SELECT route_id, title, description, matchers_json, core_json, chain_json FROM routes`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	queryLower := strings.ToLower(query)
	bestScore := 0
	var best Route
	for rows.Next() {
		var route Route
		var matchersJSON, coreJSON, chainJSON string
		if err := rows.Scan(&route.ID, &route.Title, &route.Description, &matchersJSON, &coreJSON, &chainJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(matchersJSON), &route.Matchers); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(coreJSON), &route.Core); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(chainJSON), &route.Chain); err != nil {
			return nil, err
		}

		score := 0
		for _, matcher := range route.Matchers {
			if strings.Contains(queryLower, strings.ToLower(matcher)) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = route
		}
	}
	if bestScore == 0 {
		return nil, nil
	}
	return &best, nil
}

// GetSpecSection returns the complete body of a section by heading or pattern id.
// For heading-only pattern shells, it falls back to the indexed summary.
func GetSpecSection(db *sql.DB, headingOrPattern string) (string, error) {
	headingOrPattern = strings.TrimSpace(headingOrPattern)
	normalizedPatternID := normalizePatternID(headingOrPattern)

	var body string
	var summary string
	err := db.QueryRow(`
		SELECT body, summary
		FROM sections
		WHERE heading = ? OR (? <> '' AND pattern_id = ?)
		ORDER BY CASE WHEN (? <> '' AND pattern_id = ?) THEN 0 ELSE 1 END
		LIMIT 1
	`, headingOrPattern, normalizedPatternID, normalizedPatternID, normalizedPatternID, normalizedPatternID).Scan(&body, &summary)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) != "" {
		return body, nil
	}
	return summary, nil
}

// GetSpecMeta reads a value from the meta table.
func GetSpecMeta(db *sql.DB, key string) (string, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

// GetSpecIndexInfo reads the known build provenance keys from the meta table.
func GetSpecIndexInfo(db *sql.DB) (SpecIndexInfo, error) {
	info := SpecIndexInfo{}
	readers := []struct {
		key    string
		target *string
	}{
		{key: "fpf_commit", target: &info.Commit},
		{key: "indexed_sections", target: &info.IndexedSections},
		{key: "build_time", target: &info.BuildTime},
		{key: "spec_path", target: &info.SpecPath},
		{key: "schema_version", target: &info.SchemaVersion},
	}

	for _, reader := range readers {
		value, err := GetSpecMeta(db, reader.key)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return SpecIndexInfo{}, err
		}
		*reader.target = value
	}

	return info, nil
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func normalizeChunkForIndex(chunk SpecChunk) SpecChunk {
	chunk.PatternID = normalizePatternID(chunk.PatternID)
	chunk.ParentPatternID = normalizePatternID(chunk.ParentPatternID)
	chunk.Summary = firstNonEmpty(chunk.Summary, buildSectionSummary(chunk.Heading, chunk.Body))
	chunk.Summary = cleanMarkdownText(chunk.Summary)

	for index, relatedID := range chunk.RelatedIDs {
		chunk.RelatedIDs[index] = normalizePatternID(relatedID)
	}
	chunk.RelatedIDs = dedupeStrings(chunk.RelatedIDs)

	for index, edge := range chunk.Edges {
		chunk.Edges[index].FromPatternID = normalizePatternID(edge.FromPatternID)
		chunk.Edges[index].ToPatternID = normalizePatternID(edge.ToPatternID)
	}
	chunk.Edges = appendUniqueEdges(nil, chunk.Edges...)

	if chunk.PatternID != "" {
		chunk.Aliases = appendUnique(chunk.Aliases, chunk.PatternID)
	}
	chunk.Aliases = normalizeAliases(chunk.Aliases)

	return chunk
}

func normalizeChunksForIndex(chunks []SpecChunk) []SpecChunk {
	normalized := make([]SpecChunk, 0, len(chunks))

	for _, chunk := range chunks {
		normalized = append(normalized, normalizeChunkForIndex(chunk))
	}

	return normalized
}

func normalizeRoutesForIndex(routes []Route) []Route {
	normalized := make([]Route, 0, len(routes))

	for _, route := range routes {
		normalized = append(normalized, normalizeRoute(route))
	}

	return normalized
}
