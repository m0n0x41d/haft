package fpf

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	treeDrillDownSeedMultiplier = 4
	treeDrillDownSeedFloor      = 8
	treeDrillDownSeedCeiling    = 24
)

var treeDrillDownStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "do": {}, "for": {}, "how": {},
	"i": {}, "in": {}, "is": {}, "it": {}, "need": {}, "of": {}, "or": {},
	"should": {}, "the": {}, "to": {}, "what": {}, "with": {},
}

type drillDownSection struct {
	PatternID       string
	Heading         string
	Summary         string
	ParentPatternID string
}

type drillDownBranch struct {
	LeafPatternID string
	Path          []drillDownSection
	Score         int
	SeedOrder     int
}

func searchTreeDrillDown(db *sql.DB, query string, limit int) ([]SpecSearchResult, error) {
	seedResults, err := loadDrillDownSeeds(db, query, drillDownSeedLimit(limit))
	if err != nil {
		return nil, err
	}
	if len(seedResults) == 0 {
		return nil, nil
	}

	sections, err := loadDrillDownSections(db)
	if err != nil {
		return nil, err
	}

	queryTokens := tokenizeDrillDownText(query)
	branches := buildDrillDownBranches(seedResults, sections, queryTokens)
	if len(branches) == 0 {
		return nil, nil
	}

	sortDrillDownBranches(branches)
	return buildDrillDownResults(branches, limit), nil
}

func loadDrillDownSeeds(db *sql.DB, query string, limit int) ([]SpecSearchResult, error) {
	seeds := make([]SpecSearchResult, 0, limit)
	seen := make(map[string]struct{}, limit)

	appendSeed := func(result SpecSearchResult) {
		if len(seeds) >= limit {
			return
		}
		if strings.TrimSpace(result.PatternID) == "" {
			return
		}
		if _, ok := seen[result.PatternID]; ok {
			return
		}
		seen[result.PatternID] = struct{}{}
		seeds = append(seeds, result)
	}

	route, err := classifyRoute(db, query)
	if err != nil {
		return nil, err
	}
	if route != nil {
		for _, patternID := range uniqueRoutePatternIDs(*route) {
			result, err := searchByPatternID(db, patternID)
			if err != nil {
				continue
			}
			appendSeed(result)
		}
	}

	ftsResults, err := searchFTS(db, query, limit)
	if err != nil {
		return nil, err
	}
	for _, result := range ftsResults {
		appendSeed(result)
	}

	return seeds, nil
}

func uniqueRoutePatternIDs(route Route) []string {
	patternIDs := make([]string, 0, len(route.Core)+len(route.Chain))
	seen := make(map[string]struct{}, len(route.Core)+len(route.Chain))
	appendPatternID := func(patternID string) {
		patternID = normalizePatternID(patternID)
		if patternID == "" {
			return
		}
		if _, ok := seen[patternID]; ok {
			return
		}
		seen[patternID] = struct{}{}
		patternIDs = append(patternIDs, patternID)
	}

	for _, patternID := range route.Core {
		appendPatternID(patternID)
	}
	for _, patternID := range route.Chain {
		appendPatternID(patternID)
	}

	return patternIDs
}

func loadDrillDownSections(db *sql.DB) (map[string]drillDownSection, error) {
	rows, err := db.Query(`
		SELECT pattern_id, heading, summary, parent_pattern_id
		FROM sections
		WHERE pattern_id IS NOT NULL AND pattern_id <> ''
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	sections := map[string]drillDownSection{}
	for rows.Next() {
		var section drillDownSection
		var parentPatternID sql.NullString
		if err := rows.Scan(&section.PatternID, &section.Heading, &section.Summary, &parentPatternID); err != nil {
			return nil, err
		}
		section.PatternID = normalizePatternID(section.PatternID)
		section.ParentPatternID = normalizePatternID(parentPatternID.String)
		sections[section.PatternID] = section
	}

	return sections, rows.Err()
}

func buildDrillDownBranches(
	seeds []SpecSearchResult,
	sections map[string]drillDownSection,
	queryTokens []string,
) []drillDownBranch {
	branches := make([]drillDownBranch, 0, len(seeds))
	for seedOrder, seed := range seeds {
		path := buildDrillDownPath(sections, seed.PatternID)
		if len(path) == 0 {
			continue
		}

		branches = append(branches, drillDownBranch{
			LeafPatternID: seed.PatternID,
			Path:          path,
			Score:         scoreDrillDownPath(path, queryTokens, seedOrder),
			SeedOrder:     seedOrder,
		})
	}

	return dedupeDrillDownBranches(branches)
}

func buildDrillDownPath(sections map[string]drillDownSection, leafPatternID string) []drillDownSection {
	path := make([]drillDownSection, 0, 4)
	visited := map[string]struct{}{}
	currentPatternID := normalizePatternID(leafPatternID)

	for currentPatternID != "" {
		if _, ok := visited[currentPatternID]; ok {
			break
		}
		visited[currentPatternID] = struct{}{}

		section, ok := sections[currentPatternID]
		if !ok {
			break
		}
		path = append(path, section)
		currentPatternID = section.ParentPatternID
	}

	return path
}

func dedupeDrillDownBranches(branches []drillDownBranch) []drillDownBranch {
	deduped := make(map[string]drillDownBranch, len(branches))
	for _, branch := range branches {
		existing, ok := deduped[branch.LeafPatternID]
		if ok && !drillDownBranchLess(branch, existing) {
			continue
		}
		deduped[branch.LeafPatternID] = branch
	}

	results := make([]drillDownBranch, 0, len(deduped))
	for _, branch := range deduped {
		results = append(results, branch)
	}

	return results
}

func sortDrillDownBranches(branches []drillDownBranch) {
	sort.Slice(branches, func(i, j int) bool {
		return drillDownBranchLess(branches[i], branches[j])
	})
}

func drillDownBranchLess(left, right drillDownBranch) bool {
	if left.Score != right.Score {
		return left.Score > right.Score
	}
	if left.SeedOrder != right.SeedOrder {
		return left.SeedOrder < right.SeedOrder
	}
	if len(left.Path) != len(right.Path) {
		return len(left.Path) > len(right.Path)
	}
	return left.LeafPatternID < right.LeafPatternID
}

func buildDrillDownResults(branches []drillDownBranch, limit int) []SpecSearchResult {
	if limit <= 0 {
		limit = DefaultSpecSearchLimit
	}

	results := make([]SpecSearchResult, 0, limit)
	seen := make(map[string]struct{}, limit)
	maxDepth := 0
	for _, branch := range branches {
		if len(branch.Path) > maxDepth {
			maxDepth = len(branch.Path)
		}
	}

	for depth := 0; depth < maxDepth; depth++ {
		for _, branch := range branches {
			if len(results) >= limit {
				return results
			}
			if depth >= len(branch.Path) {
				continue
			}

			section := branch.Path[depth]
			if _, ok := seen[section.PatternID]; ok {
				continue
			}
			seen[section.PatternID] = struct{}{}

			results = append(results, SpecSearchResult{
				PatternID: section.PatternID,
				Heading:   section.Heading,
				Summary:   section.Summary,
				Snippet:   firstNonEmpty(section.Summary, section.Heading),
				Rank:      -750 + float64(len(results)),
				Tier:      SpecSearchTierDrillDown,
				Reason:    formatDrillDownReason(section, branch.LeafPatternID, depth),
			})
		}
	}

	return results
}

func formatDrillDownReason(section drillDownSection, leafPatternID string, depth int) string {
	if depth == 0 || section.PatternID == leafPatternID {
		return fmt.Sprintf("tree drill-down leaf %s", leafPatternID)
	}
	return fmt.Sprintf("tree drill-down ancestor of %s", leafPatternID)
}

func scoreDrillDownPath(path []drillDownSection, queryTokens []string, seedOrder int) int {
	score := 0
	for depth, section := range path {
		weight := len(path) - depth
		score += weight * scoreDrillDownSection(section, queryTokens)
	}

	score += max(0, treeDrillDownSeedCeiling-seedOrder)
	score += len(path)
	return score
}

func scoreDrillDownSection(section drillDownSection, queryTokens []string) int {
	if len(queryTokens) == 0 {
		return 1
	}

	documentTokens := tokenSet(tokenizeDrillDownText(section.Heading + " " + section.Summary))
	if len(documentTokens) == 0 {
		return 0
	}

	score := 0
	for _, token := range queryTokens {
		if _, ok := documentTokens[token]; ok {
			score++
		}
	}

	heading := normalizeDrillDownText(section.Heading)
	summary := normalizeDrillDownText(section.Summary)
	for _, token := range queryTokens {
		switch {
		case strings.Contains(heading, token):
			score += 2
		case strings.Contains(summary, token):
			score++
		}
	}

	return score
}

func drillDownSeedLimit(limit int) int {
	if limit <= 0 {
		limit = DefaultSpecSearchLimit
	}

	seedLimit := limit * treeDrillDownSeedMultiplier
	if seedLimit < treeDrillDownSeedFloor {
		return treeDrillDownSeedFloor
	}
	if seedLimit > treeDrillDownSeedCeiling {
		return treeDrillDownSeedCeiling
	}

	return seedLimit
}

func normalizeDrillDownText(text string) string {
	text = strings.ToLower(cleanMarkdownText(text))
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return r
		}
		return ' '
	}, text)
	return strings.Join(strings.Fields(normalized), " ")
}

func tokenizeDrillDownText(text string) []string {
	normalized := normalizeDrillDownText(text)
	rawTokens := strings.Fields(normalized)
	filtered := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		if len(token) == 1 && !unicode.IsDigit(rune(token[0])) {
			continue
		}
		if _, ok := treeDrillDownStopWords[token]; ok {
			continue
		}
		filtered = append(filtered, token)
	}
	if len(filtered) > 0 {
		return dedupeStrings(filtered)
	}
	return dedupeStrings(rawTokens)
}

func tokenSet(tokens []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		set[token] = struct{}{}
	}
	return set
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
