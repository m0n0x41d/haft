package fpf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	PatternAtlasLintLeadingSpace = "leading_space_heading"
)

// PatternAtlas is a deterministic structural index over the FPF markdown.
// It is a source-card substrate only: it never accepts Haft-authored
// applicability routes and never strengthens retrieval into evidence,
// approval, or gate passage.
type PatternAtlas struct {
	SourceRef string
	FPFCommit string
	LineCount int
	Nodes     []PatternAtlasNode
	Cards     []PatternAtlasCard
	Lints     []PatternAtlasLint
}

type PatternAtlasNode struct {
	NodeID       string
	PatternID    string
	Heading      string
	Level        int
	StartLine    int
	EndLine      int
	OwnEndLine   int
	ParentNodeID string
	Path         string
	Body         string
	ContentHash  string
	SourceRef    string
	FPFCommit    string
}

type PatternAtlasCard struct {
	PatternID     string
	Title         string
	CardStartLine int
	CardEndLine   int
	RootNodeID    string
	ContentHash   string
	SourceRef     string
	FPFCommit     string
}

type PatternAtlasLint struct {
	LineNumber int
	LintKind   string
	Message    string
	RawLine    string
	SourceRef  string
	FPFCommit  string
}

func LoadPatternAtlas(path, fpfCommit string) (PatternAtlas, error) {
	markdown, err := os.ReadFile(path)
	if err != nil {
		return PatternAtlas{}, fmt.Errorf("read pattern atlas source: %w", err)
	}
	return BuildPatternAtlas(markdown, path, fpfCommit)
}

func BuildPatternAtlas(markdown []byte, sourceRef, fpfCommit string) (PatternAtlas, error) {
	lines := splitPatternAtlasLines(markdown)
	nodes, lints := parsePatternAtlasNodes(lines, sourceRef, fpfCommit)
	cards := buildPatternAtlasCards(nodes, lines, sourceRef, fpfCommit)

	return PatternAtlas{
		SourceRef: sourceRef,
		FPFCommit: fpfCommit,
		LineCount: len(lines),
		Nodes:     nodes,
		Cards:     cards,
		Lints:     lints,
	}, nil
}

func splitPatternAtlasLines(markdown []byte) []string {
	normalized := strings.ReplaceAll(string(markdown), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func parsePatternAtlasNodes(lines []string, sourceRef, fpfCommit string) ([]PatternAtlasNode, []PatternAtlasLint) {
	type parsedHeading struct {
		nodeID     string
		parentID   string
		path       string
		patternID  string
		heading    string
		level      int
		startLine  int
		leadingPad bool
		rawLine    string
	}

	headings := make([]parsedHeading, 0)
	lints := make([]PatternAtlasLint, 0)
	stack := make([]int, 0)

	for index, line := range lines {
		level, heading, leadingPad, ok := parsePatternAtlasHeading(line)
		if !ok {
			continue
		}

		for len(stack) > 0 && headings[stack[len(stack)-1]].level >= level {
			stack = stack[:len(stack)-1]
		}

		nodeID := fmt.Sprintf("%04d", len(headings))
		parentID := ""
		path := nodeID
		if len(stack) > 0 {
			parent := headings[stack[len(stack)-1]]
			parentID = parent.nodeID
			path = parent.path + "/" + nodeID
		}

		if leadingPad {
			lints = append(lints, PatternAtlasLint{
				LineNumber: index + 1,
				LintKind:   PatternAtlasLintLeadingSpace,
				Message:    "markdown heading has leading spaces; normalized for atlas extraction",
				RawLine:    line,
				SourceRef:  sourceRef,
				FPFCommit:  fpfCommit,
			})
		}

		headings = append(headings, parsedHeading{
			nodeID:     nodeID,
			parentID:   parentID,
			path:       path,
			patternID:  extractPatternID(heading),
			heading:    heading,
			level:      level,
			startLine:  index + 1,
			leadingPad: leadingPad,
			rawLine:    line,
		})
		stack = append(stack, len(headings)-1)
	}

	nodes := make([]PatternAtlasNode, 0, len(headings))
	for index, heading := range headings {
		ownEndLine := len(lines)
		if index+1 < len(headings) {
			ownEndLine = headings[index+1].startLine - 1
		}

		endLine := len(lines)
		for next := index + 1; next < len(headings); next++ {
			if headings[next].level <= heading.level {
				endLine = headings[next].startLine - 1
				break
			}
		}

		body := patternAtlasLineRange(lines, heading.startLine, ownEndLine)
		nodes = append(nodes, PatternAtlasNode{
			NodeID:       heading.nodeID,
			PatternID:    heading.patternID,
			Heading:      heading.heading,
			Level:        heading.level,
			StartLine:    heading.startLine,
			EndLine:      endLine,
			OwnEndLine:   ownEndLine,
			ParentNodeID: heading.parentID,
			Path:         heading.path,
			Body:         body,
			ContentHash:  patternAtlasHash(body),
			SourceRef:    sourceRef,
			FPFCommit:    fpfCommit,
		})
	}

	return nodes, lints
}

func parsePatternAtlasHeading(line string) (level int, heading string, leadingSpace bool, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	leadingSpace = trimmed != line && strings.HasPrefix(trimmed, "#")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false, false
	}
	level, heading, ok = parseMarkdownHeading(trimmed)
	return level, heading, leadingSpace, ok
}

func buildPatternAtlasCards(nodes []PatternAtlasNode, lines []string, sourceRef, fpfCommit string) []PatternAtlasCard {
	cards := make([]PatternAtlasCard, 0)
	for _, node := range nodes {
		if !isPatternAtlasCardRoot(node) {
			continue
		}
		body := patternAtlasLineRange(lines, node.StartLine, node.EndLine)
		title := firstNonEmpty(stripHeadingPatternID(node.Heading, node.PatternID), cleanMarkdownText(node.Heading))
		cards = append(cards, PatternAtlasCard{
			PatternID:     node.PatternID,
			Title:         title,
			CardStartLine: node.StartLine,
			CardEndLine:   node.EndLine,
			RootNodeID:    node.NodeID,
			ContentHash:   patternAtlasHash(body),
			SourceRef:     sourceRef,
			FPFCommit:     fpfCommit,
		})
	}

	sort.Slice(cards, func(i, j int) bool {
		return cards[i].CardStartLine < cards[j].CardStartLine
	})
	return cards
}

func isPatternAtlasCardRoot(node PatternAtlasNode) bool {
	if node.Level != 2 || node.PatternID == "" {
		return false
	}
	return !strings.Contains(node.PatternID, ":")
}

func patternAtlasLineRange(lines []string, startLine, endLine int) string {
	if startLine <= 0 || endLine < startLine || startLine > len(lines) {
		return ""
	}
	endLine = min(endLine, len(lines))
	return strings.Join(lines[startLine-1:endLine], "\n")
}

func patternAtlasHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
