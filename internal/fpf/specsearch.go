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
	Snippet   string
	Rank      float64
	Tier      string
	Reason    string
}

type Route struct {
	ID          string
	Title       string
	Description string
	Matchers    []string
	Core        []string
	Chain       []string
}

// SpecIndexSchemaVersion identifies the current SQLite index layout contract.
const SpecIndexSchemaVersion = "1"

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
	SpecEdgeTypeRelated,
	SpecEdgeTypeConstrains,
	SpecEdgeTypeInforms,
	SpecEdgeTypeUsedBy,
	SpecEdgeTypeRefines,
	SpecEdgeTypeSpecialisedBy,
}

type relatedExpansionPolicy struct {
	MaxResults int
}

type relatedCandidate struct {
	FromPatternID string
	ToPatternID   string
	Heading       string
	Snippet       string
	EdgeType      SpecEdgeType
}

var defaultRelatedExpansionPolicy = relatedExpansionPolicy{
	MaxResults: relatedExpansionLimit,
}

var defaultRoutes = []Route{
	{
		ID:          "project-alignment",
		Title:       "Fastest route to concrete artifacts",
		Description: "Onboarding into practical FPF artifacts and cycles.",
		Matchers:    []string{"project", "tomorrow", "artifact", "artifacts", "onboard", "onboarding", "problem card", "drr", "uts"},
		Core:        []string{"A.0", "A.1", "B.5.1", "E.9", "F.17"},
		Chain:       []string{"A.0", "A.1", "A.2", "A.3", "B.5.1", "E.9", "F.17"},
	},
	{
		ID:          "boundary-unpacking",
		Title:       "Boundary discipline and routing",
		Description: "Boundary statements, contracts, and routing language to the right semantic landing zones.",
		Matchers:    []string{"boundary", "contract", "routing", "signature stack", "admissibility", "deontic", "service", "promise"},
		Core:        []string{"A.6", "A.6.B", "A.6.C"},
		Chain:       []string{"A.6", "A.6.B", "A.6.C", "A.6.8", "F.18"},
	},
	{
		ID:          "language-discovery",
		Title:       "Language-state and routing cues",
		Description: "How partial articulation becomes routed, stabilized, and handed off.",
		Matchers:    []string{"language", "cue", "route", "stabilize", "reopen", "notice", "partial", "articulation"},
		Core:        []string{"C.2.2a", "A.16", "B.4.1"},
		Chain:       []string{"C.2.2a", "A.16", "A.16.1", "A.16.2", "B.4.1", "B.5.2.0"},
	},
	{
		ID:          "comparison-selection",
		Title:       "Characterization, comparison, and selection",
		Description: "Characteristic spaces, comparison discipline, and selector mechanics.",
		Matchers:    []string{"compare", "comparison", "pareto", "selector", "selection", "characteristic", "dimension", "normalization"},
		Core:        []string{"A.17", "A.19", "G.0"},
		Chain:       []string{"A.17", "A.18", "A.19", "A.19.CN", "A.19.CPM", "G.0"},
	},
	{
		ID:          "generator-portfolio",
		Title:       "Creative generation and portfolios",
		Description: "NQD, explore/exploit, portfolios, and creative abduction.",
		Matchers:    []string{"nqd", "portfolio", "creative", "abduction", "explore", "generator", "diversity"},
		Core:        []string{"A.0", "B.5.2.1", "G.0"},
		Chain:       []string{"A.0", "B.5.2", "B.5.2.1", "C.17", "C.18", "C.19", "G.0"},
	},
	{
		ID:          "rewrite-explanation",
		Title:       "Same-entity rewrite and explanation",
		Description: "Conservative retextualization and explanation-faithful output transformations.",
		Matchers:    []string{"rewrite", "summary", "retextualization", "translation", "explanation", "same entity"},
		Core:        []string{"A.6.3.CR", "E.17.EFP"},
		Chain:       []string{"A.6.3", "A.6.3.CR", "A.6.3.RT", "E.17", "E.17.EFP"},
	},
}

// BuildSpecIndex creates a structured SQLite index from spec chunks.
func BuildSpecIndex(dbPath string, chunks []SpecChunk) error {
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

	secIns, err := tx.Prepare(`INSERT INTO sections (id, pattern_id, heading, level, body, parent_pattern_id, keywords_json, queries_json, aliases_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
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
	for _, c := range chunks {
		c = normalizeChunkForIndex(c)
		keywordsJSON := mustJSON(c.Keywords)
		queriesJSON := mustJSON(c.Queries)
		aliasesJSON := mustJSON(c.Aliases)
		if _, err := secIns.Exec(c.ID, nullIfEmpty(c.PatternID), c.Heading, c.Level, c.Body, nullIfEmpty(c.ParentPatternID), keywordsJSON, queriesJSON, aliasesJSON); err != nil {
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

	for _, route := range defaultRoutes {
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
	if limit <= 0 {
		limit = 10
	}
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

	if patternID := extractPatternID(query); patternID != "" {
		if result, err := searchByPatternID(db, patternID); err == nil {
			appendResults([]SpecSearchResult{result})
		}
	}

	route, err := classifyRoute(db, query)
	if err != nil {
		return nil, err
	}
	if route != nil {
		routeResults, err := searchRoute(db, *route)
		if err != nil {
			return nil, err
		}
		appendResults(routeResults)
		edgeResults, err := searchRelated(db, route.Core)
		if err != nil {
			return nil, err
		}
		appendResults(edgeResults)
	}

	ftsResults, err := searchFTS(db, query, limit*2)
	if err != nil {
		return nil, err
	}
	appendResults(ftsResults)

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
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func searchByPatternID(db *sql.DB, patternID string) (SpecSearchResult, error) {
	patternID = normalizePatternID(patternID)

	var result SpecSearchResult
	err := db.QueryRow(`
		SELECT pattern_id, heading, substr(body, 1, 280)
		FROM sections
		WHERE pattern_id = ?
	`, patternID).Scan(&result.PatternID, &result.Heading, &result.Snippet)
	if err != nil {
		return SpecSearchResult{}, err
	}
	result.Rank = -1000
	result.Tier = "pattern"
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
		result, err := searchByPatternID(db, patternID)
		if err != nil {
			continue
		}
		result.Rank = -500 + float64(i)
		result.Tier = "route"
		result.Reason = route.Title
		results = append(results, result)
	}
	return results, nil
}

func searchRelated(db *sql.DB, seeds []string) ([]SpecSearchResult, error) {
	if len(seeds) == 0 {
		return nil, nil
	}

	candidates, err := loadRelatedCandidates(db, seeds)
	if err != nil {
		return nil, err
	}

	selected := selectRelatedCandidates(candidates, seeds, defaultRelatedExpansionPolicy)
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
		SELECT e.from_pattern_id, e.to_pattern_id, e.edge_type, s.heading, substr(s.body, 1, 280)
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
		if err := rows.Scan(&candidate.FromPatternID, &candidate.ToPatternID, &edgeType, &candidate.Heading, &candidate.Snippet); err != nil {
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

	if len(preferred) > 0 {
		return truncateRelatedCandidates(preferred, policy.MaxResults)
	}

	return truncateRelatedCandidates(fallback, policy.MaxResults)
}

func buildRelatedResults(candidates []relatedCandidate) []SpecSearchResult {
	results := make([]SpecSearchResult, 0, len(candidates))
	for index, candidate := range candidates {
		result := SpecSearchResult{
			PatternID: candidate.ToPatternID,
			Heading:   candidate.Heading,
			Snippet:   candidate.Snippet,
			Rank:      -200 + float64(index),
			Tier:      "related",
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
		SELECT s.pattern_id, s.heading, snippet(fpf_fts, 2, '>>>', '<<<', '...', 64), rank
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
		if err := rows.Scan(&r.PatternID, &r.Heading, &r.Snippet, &r.Rank); err != nil {
			return nil, err
		}
		r.Tier = "fts"
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
func GetSpecSection(db *sql.DB, headingOrPattern string) (string, error) {
	headingOrPattern = strings.TrimSpace(headingOrPattern)
	normalizedPatternID := normalizePatternID(headingOrPattern)

	var body string
	err := db.QueryRow(`
		SELECT body
		FROM sections
		WHERE heading = ? OR (? <> '' AND pattern_id = ?)
		ORDER BY CASE WHEN (? <> '' AND pattern_id = ?) THEN 0 ELSE 1 END
		LIMIT 1
	`, headingOrPattern, normalizedPatternID, normalizedPatternID, normalizedPatternID, normalizedPatternID).Scan(&body)
	if err != nil {
		return "", err
	}
	return body, nil
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

	return chunk
}
