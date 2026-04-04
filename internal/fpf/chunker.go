package fpf

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var patternIDCandidateRE = regexp.MustCompile(`(?i)\b(?:[A-K]\d+[A-Za-z0-9.:]*|[A-K]\.[A-Za-z0-9]+(?:[.:][A-Za-z0-9]+)*)\b`)
var patternIDDigitsRE = regexp.MustCompile(`^\d+$`)
var patternIDDigitSuffixRE = regexp.MustCompile(`^(\d+)([A-Za-z]+)$`)
var patternIDWordRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
var dependencyClauseLabelRE = regexp.MustCompile(`(?:^|[.;]\s+)([A-Za-z][A-Za-z /-]+):`)
var trailingParentheticalRE = regexp.MustCompile(`^(.*)\(([^()]+)\)\s*$`)
var orderedListMarkerRE = regexp.MustCompile(`^\d+[.)]\s+`)

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
	Summary         string
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
		chunk := SpecChunk{
			ID:              id,
			Heading:         currentHeading,
			Level:           currentLevel,
			Body:            body,
			Summary:         buildSectionSummary(currentHeading, body),
			PatternID:       currentPatternID,
			ParentPatternID: currentParentPatternID,
			Aliases:         buildAliases(currentHeading, currentPatternID),
		}
		if shouldKeepChunk(chunk) {
			chunks = append(chunks, chunk)
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
	match := patternIDCandidateRE.FindString(text)
	return normalizePatternID(match)
}

func extractPatternIDs(text string) []string {
	matches := patternIDCandidateRE.FindAllString(text, -1)
	normalized := make([]string, 0, len(matches))
	for _, match := range matches {
		normalized = append(normalized, normalizePatternID(match))
	}
	return dedupeStrings(normalized)
}

func shouldKeepChunk(chunk SpecChunk) bool {
	if strings.TrimSpace(chunk.Heading) == "" {
		return false
	}
	if strings.TrimSpace(chunk.Body) != "" {
		return true
	}
	return shouldKeepHeadingOnlyPatternChunk(chunk)
}

func shouldKeepHeadingOnlyPatternChunk(chunk SpecChunk) bool {
	if chunk.PatternID == "" {
		return false
	}
	if strings.Contains(chunk.PatternID, ":") {
		return false
	}
	return chunk.Level == 2
}

func buildAliases(heading, patternID string) []string {
	var aliases []string
	if patternID != "" {
		aliases = append(aliases, patternID)
	}
	cleanHeading := normalizeAliasText(heading)
	if cleanHeading == "" {
		return normalizeAliases(aliases)
	}

	aliases = append(aliases, cleanHeading)

	title := stripHeadingPatternID(cleanHeading, patternID)
	if title != "" && title != cleanHeading {
		aliases = append(aliases, title)
	}

	for _, candidate := range []string{cleanHeading, title} {
		left, right, ok := splitAliasPair(candidate)
		if ok {
			aliases = append(aliases, left, right)
			if base, alias, ok := splitTrailingParenthetical(right); ok {
				aliases = append(aliases, base)
				if isTechnicalAlias(alias) {
					aliases = append(aliases, alias)
				}
			}
		}

		base, alias, ok := splitTrailingParenthetical(candidate)
		if ok {
			aliases = append(aliases, base)
			if isTechnicalAlias(alias) {
				aliases = append(aliases, alias)
			}
		}
	}

	return normalizeAliases(aliases)
}

func normalizeAliasText(text string) string {
	replacer := strings.NewReplacer("“", "", "”", "", "‘", "", "’", "", `"`, "", "'", "")
	text = replacer.Replace(text)
	text = cleanMarkdownText(text)
	text = strings.TrimSpace(strings.Trim(text, " -:;,.[]{}"))
	return text
}

func normalizeAliases(aliases []string) []string {
	result := make([]string, 0, len(aliases))
	seen := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		normalized := normalizeAliasText(alias)
		if normalized == "" {
			continue
		}

		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func stripHeadingPatternID(text, patternID string) string {
	if patternID == "" {
		return ""
	}

	text = normalizeAliasText(text)
	patternID = normalizeAliasText(patternID)
	if !strings.HasPrefix(text, patternID) {
		return ""
	}

	trimmed := strings.TrimSpace(strings.TrimPrefix(text, patternID))
	trimmed = strings.TrimLeft(trimmed, "-:; ")
	return normalizeAliasText(trimmed)
}

func splitAliasPair(text string) (string, string, bool) {
	text = normalizeAliasText(text)
	if text == "" {
		return "", "", false
	}

	parts := strings.SplitN(text, " - ", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	left := normalizeAliasText(parts[0])
	right := normalizeAliasText(parts[1])
	if left == "" || right == "" {
		return "", "", false
	}

	return left, right, true
}

func splitTrailingParenthetical(text string) (string, string, bool) {
	text = normalizeAliasText(text)
	if text == "" {
		return "", "", false
	}

	match := trailingParentheticalRE.FindStringSubmatch(text)
	if len(match) != 3 {
		return "", "", false
	}

	base := normalizeAliasText(match[1])
	alias := normalizeAliasText(match[2])
	if base == "" || alias == "" {
		return "", "", false
	}

	return base, alias, true
}

func isTechnicalAlias(text string) bool {
	text = normalizeAliasText(text)
	if text == "" {
		return false
	}

	if strings.Contains(text, " / ") || strings.Contains(text, " -> ") {
		return false
	}

	if extractPatternID(text) != "" {
		return true
	}

	if strings.Contains(text, " ") {
		return false
	}

	hasUpper := false
	hasMarker := false
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r == '.' || r == '-' || r == '_' || (r >= '0' && r <= '9'):
			hasMarker = true
		}
	}

	return hasUpper && hasMarker
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
	segment = normalizeQueryListText(segment)
	queries := splitQueryList(segment)
	return dedupeStrings(queries)
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

func normalizeQueryListText(text string) string {
	replacer := strings.NewReplacer(
		"“", `"`,
		"”", `"`,
		"\u00a0", " ",
	)
	text = replacer.Replace(text)
	text = strings.TrimSpace(text)
	return strings.TrimRight(text, ". ")
}

func splitQueryList(text string) []string {
	var (
		items   []string
		current strings.Builder
		inQuote bool
	)

	flush := func() {
		query := normalizeQueryItem(current.String())
		if query == "" {
			current.Reset()
			return
		}

		items = append(items, query)
		current.Reset()
	}

	for _, r := range text {
		switch {
		case r == '"':
			inQuote = !inQuote
		case !inQuote && (r == ',' || r == ';'):
			flush()
		default:
			current.WriteRune(r)
		}
	}

	flush()
	return items
}

func normalizeQueryItem(text string) string {
	replacer := strings.NewReplacer(
		"“", `"`,
		"”", `"`,
		"\u00a0", " ",
	)
	text = replacer.Replace(text)
	text = cleanMarkdownText(text)
	text = strings.TrimSpace(text)
	text = strings.Trim(text, `"`)
	text = strings.TrimSpace(text)
	text = strings.TrimRight(text, ".,;:")
	text = strings.TrimSpace(text)
	return text
}

func cleanMarkdownText(text string) string {
	replacer := strings.NewReplacer("**", "", "*", "", "`", "", "&nbsp;", " ")
	text = replacer.Replace(text)
	text = strings.ReplaceAll(text, "—", "-")
	text = strings.ReplaceAll(text, "–", "-")
	text = strings.ReplaceAll(text, "‑", "-")
	text = strings.ReplaceAll(text, "−", "-")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func buildSectionSummary(heading, body string) string {
	paragraphs := splitSummaryParagraphs(body)

	for _, paragraph := range paragraphs {
		summary := firstNonEmpty(extractQuotedSummary(paragraph), summarizeParagraph(paragraph))
		if summary != "" {
			return summary
		}
	}

	return fallbackSectionSummary(heading)
}

func splitSummaryParagraphs(body string) []string {
	lines := strings.Split(body, "\n")
	paragraphs := make([]string, 0, len(lines))
	current := make([]string, 0, 8)
	inCodeFence := false

	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(current, "\n"))
		current = current[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			flush()
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence {
			continue
		}
		if trimmed == "" {
			flush()
			continue
		}
		current = append(current, line)
	}

	flush()
	return paragraphs
}

func extractQuotedSummary(paragraph string) string {
	lines := strings.Split(paragraph, "\n")
	if len(lines) == 0 {
		return ""
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ">") {
			return ""
		}
	}

	for _, line := range lines {
		candidate := trimSummaryLinePrefix(line)
		candidate = cleanMarkdownText(candidate)
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}

		lower := strings.ToLower(candidate)
		if !strings.Contains(lower, "purpose:") && !strings.Contains(lower, "purpose (one line):") {
			continue
		}

		summary := candidate
		if index := strings.Index(candidate, ":"); index >= 0 {
			summary = strings.TrimSpace(candidate[index+1:])
		}
		summary = extractLeadingSentence(summary)
		if summary != "" {
			return summary
		}
	}

	return ""
}

func summarizeParagraph(paragraph string) string {
	if isMetadataParagraph(paragraph) || isMarkdownTableParagraph(paragraph) {
		return ""
	}

	text := collapseSummaryParagraph(paragraph)
	if text == "" {
		return ""
	}

	return firstNonEmpty(extractLeadingSentence(text), text)
}

func isMetadataParagraph(paragraph string) bool {
	lines := strings.Split(paragraph, "\n")
	if len(lines) == 0 {
		return false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ">") {
			return false
		}
	}

	return true
}

func isMarkdownTableParagraph(paragraph string) bool {
	lines := strings.Split(paragraph, "\n")
	if len(lines) == 0 {
		return false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			return false
		}
	}

	return true
}

func collapseSummaryParagraph(paragraph string) string {
	lines := strings.Split(paragraph, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = trimSummaryLinePrefix(line)
		line = cleanMarkdownText(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}

	return cleanMarkdownText(strings.Join(cleaned, " "))
}

func trimSummaryLinePrefix(line string) string {
	line = strings.TrimSpace(line)
	for {
		switch {
		case strings.HasPrefix(line, ">"):
			line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		case strings.HasPrefix(line, "- "):
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		case strings.HasPrefix(line, "* "):
			line = strings.TrimSpace(strings.TrimPrefix(line, "* "))
		case strings.HasPrefix(line, "+ "):
			line = strings.TrimSpace(strings.TrimPrefix(line, "+ "))
		case orderedListMarkerRE.MatchString(line):
			line = orderedListMarkerRE.ReplaceAllString(line, "")
			line = strings.TrimSpace(line)
		default:
			return line
		}
	}
}

func extractLeadingSentence(text string) string {
	text = cleanMarkdownText(text)
	if text == "" {
		return ""
	}

	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '.', '!', '?':
			if !isSentenceBoundary(text, index) {
				continue
			}
			end := advanceSentenceBoundary(text, index+1)
			return strings.TrimSpace(text[:end])
		}
	}

	return ""
}

func isSentenceBoundary(text string, index int) bool {
	next := advanceSentenceBoundary(text, index+1)
	return next == len(text) || text[next] == ' '
}

func advanceSentenceBoundary(text string, index int) int {
	for index < len(text) {
		switch text[index] {
		case '"', '\'', ')', ']', '}':
			index++
		default:
			return index
		}
	}
	return index
}

func fallbackSectionSummary(heading string) string {
	patternID := extractPatternID(heading)
	title := stripHeadingPatternID(heading, patternID)
	return firstNonEmpty(title, cleanMarkdownText(heading))
}

func normalizePatternID(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "\"'`.,;!?)]}")
	if text == "" {
		return ""
	}

	parts := strings.SplitN(text, ":", 2)
	base := normalizePatternBase(parts[0])
	if base == "" {
		return ""
	}

	if len(parts) == 1 {
		return base
	}

	suffix := normalizePatternPath(parts[1])
	if suffix == "" {
		return base
	}

	return base + ":" + suffix
}

func normalizePatternBase(text string) string {
	text = strings.TrimSpace(text)
	if len(text) < 2 {
		return ""
	}

	prefix := strings.ToUpper(string(text[0]))
	if !isPatternPrefix(prefix) {
		return ""
	}

	remainder := strings.TrimSpace(text[1:])
	if remainder == "" {
		return ""
	}
	if !strings.HasPrefix(remainder, ".") {
		if !patternIDDigitsRE.MatchString(string(remainder[0])) {
			return ""
		}
		remainder = "." + remainder
	}

	rawSegments := strings.Split(strings.TrimPrefix(remainder, "."), ".")
	segments := make([]string, 0, len(rawSegments))
	for _, rawSegment := range rawSegments {
		segment := normalizePatternSegment(rawSegment)
		if segment == "" {
			return ""
		}
		segments = append(segments, segment)
	}

	return prefix + "." + strings.Join(segments, ".")
}

func normalizePatternPath(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	rawSegments := strings.Split(text, ".")
	segments := make([]string, 0, len(rawSegments))
	for _, rawSegment := range rawSegments {
		segment := normalizePatternSegment(rawSegment)
		if segment == "" {
			return ""
		}
		segments = append(segments, segment)
	}

	return strings.Join(segments, ".")
}

func normalizePatternSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}

	if patternIDDigitsRE.MatchString(segment) {
		return segment
	}

	match := patternIDDigitSuffixRE.FindStringSubmatch(segment)
	if len(match) == 3 {
		return match[1] + strings.ToLower(match[2])
	}

	if patternIDWordRE.MatchString(segment) {
		return strings.ToUpper(segment)
	}

	return ""
}

func isPatternPrefix(prefix string) bool {
	return len(prefix) == 1 && prefix[0] >= 'A' && prefix[0] <= 'K'
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
