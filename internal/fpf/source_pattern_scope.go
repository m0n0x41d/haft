package fpf

import (
	"fmt"
	"regexp"
	"strings"
)

var patternScopeIDFieldRE = regexp.MustCompile(`(?i)\bPatternScopeId\b[^A-Za-z0-9]*(G\.[0-9]+:Ext\.[A-Za-z0-9_]+)\b`)
var patternExtensionIDFieldRE = regexp.MustCompile(`(?i)\bGPatternExtensionId\b[^A-Za-z0-9]*([A-Za-z0-9_]+)\b`)

type patternScopeDeclaration struct {
	sourceID        string
	parentPatternID string
	title           string
	startLine       int
	endLine         int
}

func buildPatternScopeSourceUnits(document SourceDocument, atlas PatternAtlas) ([]SourceUnit, error) {
	lines := splitPatternAtlasLines(document.Markdown)
	declarations, err := parsePatternScopeDeclarations(lines, atlas)
	if err != nil {
		return nil, err
	}

	units := make([]SourceUnit, 0, len(declarations))
	for _, declaration := range declarations {
		body := patternAtlasLineRange(lines, declaration.startLine, declaration.endLine)
		unitID := "spec:pattern_scope:" + sourceUnitSlug(declaration.sourceID)
		unit := newSourceUnit(
			unitID,
			declaration.sourceID,
			SourceUnitRolePatternScope,
			declaration.title,
			body,
			declaration.parentPatternID,
			declaration.parentPatternID,
			document,
			declaration.startLine,
			declaration.endLine,
		)
		unit.Keywords = sourceKeywords(declaration.title, body)
		units = append(units, unit)
	}
	return units, nil
}

func parsePatternScopeDeclarations(lines []string, atlas PatternAtlas) ([]patternScopeDeclaration, error) {
	declarations := make([]patternScopeDeclaration, 0)
	seen := make(map[string]int)
	for index, line := range lines {
		sourceID, found := extractPatternScopeIDField(line)
		if !found {
			continue
		}

		declarationLine := index + 1
		markerLine, markerFound := findPatternScopeMarkerLine(lines, declarationLine, sourceID)
		if !markerFound {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s lacks a GPatternExtension block marker",
				atlas.SourceRef,
				declarationLine,
				sourceID,
			)
		}

		card, cardFound := findPatternScopeCard(atlas.Cards, declarationLine)
		if !cardFound {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s is outside a pattern body",
				atlas.SourceRef,
				declarationLine,
				sourceID,
			)
		}

		parentPatternID := patternScopeParentPatternID(sourceID)
		if !strings.EqualFold(parentPatternID, card.PatternID) {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s belongs to %s, not containing pattern %s",
				atlas.SourceRef,
				declarationLine,
				sourceID,
				parentPatternID,
				card.PatternID,
			)
		}

		endLine := findPatternScopeEndLine(lines, markerLine, card.CardEndLine)
		body := patternAtlasLineRange(lines, markerLine, endLine)
		extensionID, extensionFound := extractPatternExtensionIDField(body)
		if !extensionFound {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s lacks GPatternExtensionId",
				atlas.SourceRef,
				declarationLine,
				sourceID,
			)
		}
		if extensionID != patternScopeExtensionID(sourceID) {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s disagrees with GPatternExtensionId %s",
				atlas.SourceRef,
				declarationLine,
				sourceID,
				extensionID,
			)
		}
		if !strings.Contains(body, "GoverningPatternId") {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s lacks GoverningPatternId",
				atlas.SourceRef,
				declarationLine,
				sourceID,
			)
		}
		if !strings.Contains(body, "GPatternExtensionKind") {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s lacks GPatternExtensionKind",
				atlas.SourceRef,
				declarationLine,
				sourceID,
			)
		}

		key := sourceReferenceKey(sourceID)
		if previousLine, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf(
				"pattern_scope_source_malformed[%s:%d]: PatternScopeId %s duplicates declaration at line %d",
				atlas.SourceRef,
				declarationLine,
				sourceID,
				previousLine,
			)
		}
		seen[key] = declarationLine

		title := patternScopeMarkerTitle(lines[markerLine-1])
		declarations = append(declarations, patternScopeDeclaration{
			sourceID:        sourceID,
			parentPatternID: card.PatternID,
			title:           firstNonEmpty(title, sourceID),
			startLine:       markerLine,
			endLine:         endLine,
		})
	}
	return declarations, nil
}

func extractPatternScopeIDField(line string) (string, bool) {
	match := patternScopeIDFieldRE.FindStringSubmatch(line)
	if len(match) != 2 {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func extractPatternExtensionIDField(body string) (string, bool) {
	match := patternExtensionIDFieldRE.FindStringSubmatch(body)
	if len(match) != 2 {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func findPatternScopeMarkerLine(lines []string, declarationLine int, sourceID string) (int, bool) {
	firstCandidate := declarationLine - 6
	if firstCandidate < 1 {
		firstCandidate = 1
	}
	for lineNumber := declarationLine; lineNumber >= firstCandidate; lineNumber-- {
		line := lines[lineNumber-1]
		if isPatternScopeMarker(line) || isPatternScopeIDHeading(line, sourceID) {
			return lineNumber, true
		}
	}
	return 0, false
}

func isPatternScopeIDHeading(line, sourceID string) bool {
	_, heading, _, isHeading := parsePatternAtlasHeading(line)
	if !isHeading {
		return false
	}
	headingKey := sourceReferenceKey(heading)
	sourceKey := sourceReferenceKey(sourceID)
	return strings.Contains(headingKey, sourceKey)
}

func isPatternScopeMarker(line string) bool {
	clean := patternScopeMarkerTitle(line)
	lower := strings.ToLower(clean)
	_, _, _, isHeading := parsePatternAtlasHeading(line)
	if isHeading && strings.Contains(lower, "gpatternextension") {
		return true
	}
	markers := []string{
		"gpatternextension:",
		"gpatternextension block:",
		"gpatternextension -",
		"gpatternextension —",
	}
	for _, marker := range markers {
		if strings.HasPrefix(lower, marker) {
			return true
		}
	}
	return false
}

func patternScopeMarkerTitle(line string) string {
	_, heading, _, isHeading := parsePatternAtlasHeading(line)
	if isHeading {
		return cleanMarkdownText(heading)
	}
	return cleanMarkdownText(line)
}

func findPatternScopeCard(cards []PatternAtlasCard, lineNumber int) (PatternAtlasCard, bool) {
	for _, card := range cards {
		if lineNumber >= card.CardStartLine && lineNumber <= card.CardEndLine {
			return card, true
		}
	}
	return PatternAtlasCard{}, false
}

func findPatternScopeEndLine(lines []string, markerLine, cardEndLine int) int {
	markerLevel, _, _, markerIsHeading := parsePatternAtlasHeading(lines[markerLine-1])
	for lineNumber := markerLine + 1; lineNumber <= cardEndLine; lineNumber++ {
		line := lines[lineNumber-1]
		if isPatternScopeMarker(line) {
			return lineNumber - 1
		}

		level, _, _, isHeading := parsePatternAtlasHeading(line)
		if !isHeading {
			continue
		}
		if !markerIsHeading || level <= markerLevel {
			return lineNumber - 1
		}
	}
	return cardEndLine
}

func patternScopeParentPatternID(sourceID string) string {
	parts := strings.SplitN(sourceID, ":", 2)
	return normalizePatternID(parts[0])
}

func patternScopeExtensionID(sourceID string) string {
	const marker = ":Ext."
	index := strings.Index(sourceID, marker)
	if index < 0 {
		return ""
	}
	return sourceID[index+len(marker):]
}
