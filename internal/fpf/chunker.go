package fpf

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var patternIDRE = regexp.MustCompile(`\b(?:[A-K]\.\d+(?:\.\d+)*(?:\.[A-Z]+)?(?:[:]\d+(?:\.\d+)*)?|[A-K]\.\d+(?:\.\d+)*(?:\.[A-Z]+)?)\b`)
var quotedQueryRE = regexp.MustCompile(`"([^"]+)"`)
var dependencyClauseLabelRE = regexp.MustCompile(`([A-Za-z][A-Za-z /-]+):`)

type SpecEdgeType string

const (
	SpecEdgeTypeRelated         SpecEdgeType = "related"
	SpecEdgeTypeBuildsOn        SpecEdgeType = "builds_on"
	SpecEdgeTypePrerequisiteFor SpecEdgeType = "prerequisite_for"
	SpecEdgeTypeCoordinatesWith SpecEdgeType = "coordinates_with"
	SpecEdgeTypeConstrains      SpecEdgeType = "constrains"
	SpecEdgeTypeInforms         SpecEdgeType = "informs"
	SpecEdgeTypeUsedBy          SpecEdgeType = "used_by"
	SpecEdgeTypeRefines         SpecEdgeType = "refines"
	SpecEdgeTypeSpecialisedBy   SpecEdgeType = "specialised_by"
)

type SpecEdge struct {
	FromPatternID string
	ToPatternID   string
	EdgeType      SpecEdgeType
}

// SpecChunk represents a section of the FPF spec.
type SpecChunk struct {
	ID              int
	Heading         string
	Level           int
	Body            string
	PatternID       string
	ParentPatternID string
	Keywords        []string
	Queries         []string
	Aliases         []string
	RelatedIDs      []string
	Edges           []SpecEdge
}

// SpecCatalogEntry carries metadata parsed from the specification tables of contents.
type SpecCatalogEntry struct {
	PatternID  string
	Title      string
	Keywords   []string
	Queries    []string
	RelatedIDs []string
	Edges      []SpecEdge
}

// ChunkMarkdown splits a markdown document into chunks by headings.
// Each chunk contains the heading and all content until the next heading
// of the same or higher level.
func ChunkMarkdown(r io.Reader) ([]SpecChunk, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	type headingFrame struct {
		level     int
		patternID string
	}

	var chunks []SpecChunk
	var stack []headingFrame
	var currentHeading string
	var currentLevel int
	var currentBody strings.Builder
	var currentPatternID string
	var currentParentPatternID string
	id := 0

	flush := func() {
		body := strings.TrimSpace(currentBody.String())
		if currentHeading != "" && body != "" {
			chunks = append(chunks, SpecChunk{
				ID:              id,
				Heading:         currentHeading,
				Level:           currentLevel,
				Body:            body,
				PatternID:       currentPatternID,
				ParentPatternID: currentParentPatternID,
				Aliases:         buildAliases(currentHeading, currentPatternID),
			})
			id++
		}
		currentBody.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if level, heading, ok := parseMarkdownHeading(line); ok {
			flush()

			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}

			currentParentPatternID = ""
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].patternID != "" {
					currentParentPatternID = stack[i].patternID
					break
				}
			}

			currentHeading = heading
			currentLevel = level
			currentPatternID = extractPatternID(heading)
			stack = append(stack, headingFrame{level: level, patternID: currentPatternID})
		} else if currentHeading != "" {
			currentBody.WriteString(line)
			currentBody.WriteString("\n")
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning markdown: %w", err)
	}
	return chunks, nil
}

// ParseSpecCatalog extracts search-oriented metadata from the table rows in FPF-Spec.md.
func ParseSpecCatalog(r io.Reader) (map[string]SpecCatalogEntry, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	catalog := make(map[string]SpecCatalogEntry)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			continue
		}

		cells := splitMarkdownTableRow(line)
		if len(cells) < 2 || isMarkdownSeparatorRow(cells) {
			continue
		}

		id := cleanMarkdownText(cells[0])
		title := ""
		searchCell := ""
		depsCell := ""

		switch {
		case len(cells) >= 5 && extractPatternID(id) != "":
			title = cleanMarkdownText(cells[1])
			searchCell = cleanMarkdownText(cells[3])
			depsCell = cleanMarkdownText(cells[4])
		case len(cells) >= 4 && extractPatternID(cleanMarkdownText(cells[1])) != "":
			id = extractPatternID(cleanMarkdownText(cells[1]))
			title = cleanMarkdownText(cells[1])
			searchCell = cleanMarkdownText(cells[3])
			if len(cells) >= 5 {
				depsCell = cleanMarkdownText(cells[4])
			}
		default:
			continue
		}

		patternID := extractPatternID(id)
		if patternID == "" || strings.Contains(strings.ToLower(patternID), "cluster") {
			continue
		}

		entry := catalog[patternID]
		entry.PatternID = patternID
		entry.Title = firstNonEmpty(entry.Title, title)
		entry.Keywords = appendUnique(entry.Keywords, parseKeywords(searchCell)...)
		entry.Queries = appendUnique(entry.Queries, parseQueries(searchCell)...)
		typedEdges, relatedIDs := parseDependencyEdges(patternID, depsCell)
		entry.Edges = appendUniqueEdges(entry.Edges, typedEdges...)
		entry.RelatedIDs = appendUnique(entry.RelatedIDs, relatedIDs...)
		catalog[patternID] = entry
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan catalog: %w", err)
	}
	return catalog, nil
}

// EnrichChunks overlays table-of-contents metadata onto parsed sections.
func EnrichChunks(chunks []SpecChunk, catalog map[string]SpecCatalogEntry) []SpecChunk {
	result := make([]SpecChunk, len(chunks))
	copy(result, chunks)
	for i := range result {
		if result[i].PatternID == "" {
			continue
		}
		entry, ok := catalog[result[i].PatternID]
		if !ok {
			continue
		}
		result[i].Keywords = appendUnique(result[i].Keywords, entry.Keywords...)
		result[i].Queries = appendUnique(result[i].Queries, entry.Queries...)
		result[i].RelatedIDs = appendUnique(result[i].RelatedIDs, entry.RelatedIDs...)
		result[i].Edges = appendUniqueEdges(result[i].Edges, entry.Edges...)
		result[i].Aliases = appendUnique(result[i].Aliases, buildAliases(entry.Title, entry.PatternID)...)
		result[i].Aliases = appendUnique(result[i].Aliases, entry.Title)
	}
	return result
}

func parseMarkdownHeading(line string) (level int, text string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i > 6 || i >= len(trimmed) || trimmed[i] != ' ' {
		return 0, "", false
	}
	return i, strings.TrimSpace(trimmed[i+1:]), true
}

func extractPatternID(text string) string {
	match := patternIDRE.FindString(text)
	return strings.TrimSpace(match)
}

func extractPatternIDs(text string) []string {
	matches := patternIDRE.FindAllString(text, -1)
	return dedupeStrings(matches)
}

func buildAliases(heading, patternID string) []string {
	var aliases []string
	if patternID != "" {
		aliases = append(aliases, patternID)
	}
	cleanHeading := cleanMarkdownText(heading)
	if cleanHeading != "" {
		aliases = append(aliases, cleanHeading)
	}
	return dedupeStrings(aliases)
}

func splitMarkdownTableRow(line string) []string {
	trimmed := strings.TrimSpace(strings.Trim(line, "|"))
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func isMarkdownSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		clean := strings.Trim(cell, ":- ")
		if clean != "" {
			return false
		}
	}
	return true
}

func parseKeywords(text string) []string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	lower := strings.ToLower(text)
	start := strings.Index(lower, "keywords:")
	if start == -1 {
		return nil
	}
	segment := text[start+len("keywords:"):]
	if idx := strings.Index(strings.ToLower(segment), "queries:"); idx >= 0 {
		segment = segment[:idx]
	}
	return splitList(segment)
}

func parseQueries(text string) []string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	lower := strings.ToLower(text)
	start := strings.Index(lower, "queries:")
	if start == -1 {
		return nil
	}
	segment := text[start+len("queries:"):]
	quoted := quotedQueryRE.FindAllStringSubmatch(segment, -1)
	if len(quoted) > 0 {
		queries := make([]string, 0, len(quoted))
		for _, match := range quoted {
			queries = append(queries, strings.TrimSpace(match[1]))
		}
		return dedupeStrings(queries)
	}
	return splitList(segment)
}

func splitList(text string) []string {
	text = cleanMarkdownText(text)
	parts := strings.Split(text, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"'`))
		if part != "" {
			items = append(items, part)
		}
	}
	return dedupeStrings(items)
}

func cleanMarkdownText(text string) string {
	replacer := strings.NewReplacer("**", "", "*", "", "`", "", "&nbsp;", " ")
	text = replacer.Replace(text)
	text = strings.ReplaceAll(text, "—", "-")
	text = strings.ReplaceAll(text, "–", "-")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func appendUnique(dst []string, items ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, item := range dst {
		seen[item] = struct{}{}
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}

func dedupeStrings(items []string) []string {
	var result []string
	for _, item := range items {
		result = appendUnique(result, item)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type dependencyEdgeRule struct {
	EdgeType SpecEdgeType
}

var dependencyEdgeRules = map[string]dependencyEdgeRule{
	"builds on":             {EdgeType: SpecEdgeTypeBuildsOn},
	"depends on":            {EdgeType: SpecEdgeTypeBuildsOn},
	"prerequisite for":      {EdgeType: SpecEdgeTypePrerequisiteFor},
	"is a prerequisite for": {EdgeType: SpecEdgeTypePrerequisiteFor},
	"coordinates with":      {EdgeType: SpecEdgeTypeCoordinatesWith},
	"constrains":            {EdgeType: SpecEdgeTypeConstrains},
	"informs":               {EdgeType: SpecEdgeTypeInforms},
	"used by":               {EdgeType: SpecEdgeTypeUsedBy},
	"is used by":            {EdgeType: SpecEdgeTypeUsedBy},
	"refines":               {EdgeType: SpecEdgeTypeRefines},
	"specialised by":        {EdgeType: SpecEdgeTypeSpecialisedBy},
	"is specialised by":     {EdgeType: SpecEdgeTypeSpecialisedBy},
	"specialized by":        {EdgeType: SpecEdgeTypeSpecialisedBy},
	"is specialized by":     {EdgeType: SpecEdgeTypeSpecialisedBy},
}

type dependencyClause struct {
	Label string
	Value string
}

func parseDependencyEdges(patternID, text string) ([]SpecEdge, []string) {
	text = cleanMarkdownText(text)
	if text == "" {
		return nil, nil
	}

	clauses := parseDependencyClauses(text)
	if len(clauses) == 0 {
		return nil, extractPatternIDs(text)
	}

	var edges []SpecEdge
	var relatedIDs []string
	for _, clause := range clauses {
		patternIDs := extractPatternIDs(clause.Value)
		if len(patternIDs) == 0 {
			continue
		}

		rule, ok := dependencyEdgeRules[normalizeDependencyLabel(clause.Label)]
		if !ok {
			relatedIDs = appendUnique(relatedIDs, patternIDs...)
			continue
		}

		for _, targetPatternID := range patternIDs {
			edge := SpecEdge{
				FromPatternID: patternID,
				ToPatternID:   targetPatternID,
				EdgeType:      rule.EdgeType,
			}
			edges = appendUniqueEdges(edges, edge)
		}
	}

	return edges, relatedIDs
}

func parseDependencyClauses(text string) []dependencyClause {
	matches := dependencyClauseLabelRE.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	clauses := make([]dependencyClause, 0, len(matches))
	for index, match := range matches {
		label := strings.TrimSpace(text[match[2]:match[3]])
		valueStart := match[1]
		valueEnd := len(text)
		if index+1 < len(matches) {
			valueEnd = matches[index+1][0]
		}

		value := strings.TrimSpace(text[valueStart:valueEnd])
		value = strings.Trim(value, ". ")
		clauses = append(clauses, dependencyClause{
			Label: label,
			Value: value,
		})
	}

	return clauses
}

func normalizeDependencyLabel(label string) string {
	label = strings.ToLower(label)
	label = strings.Join(strings.Fields(label), " ")
	return strings.TrimSpace(label)
}

func appendUniqueEdges(dst []SpecEdge, edges ...SpecEdge) []SpecEdge {
	seen := make(map[string]struct{}, len(dst))
	for _, edge := range dst {
		seen[edgeKey(edge)] = struct{}{}
	}
	for _, edge := range edges {
		if edge.FromPatternID == "" || edge.ToPatternID == "" || edge.EdgeType == "" {
			continue
		}
		key := edgeKey(edge)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dst = append(dst, edge)
	}
	return dst
}

func edgeKey(edge SpecEdge) string {
	return edge.FromPatternID + "|" + edge.ToPatternID + "|" + string(edge.EdgeType)
}
