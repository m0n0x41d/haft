package fpf

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	PatternAtlasBodyKindFullCardRange = "full_card_range"
	PatternAtlasLintLeadingSpace      = "leading_space_heading"
)

// PatternAtlas is a deterministic structural index over the FPF markdown.
// It is a source-card substrate only: it never accepts PatternUse routes and
// never strengthens retrieval into evidence, approval, or gate passage.
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

type PatternAtlasCardContent struct {
	PatternID   string `json:"pattern_id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	RootNodeID  string `json:"root_node_id"`
	ContentHash string `json:"content_hash"`
	SourceRef   string `json:"source_ref"`
	FPFCommit   string `json:"fpf_commit"`
	NodeCount   int    `json:"node_count"`
	BodyKind    string `json:"body_kind"`
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

func StorePatternAtlas(dbPath string, atlas PatternAtlas) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open atlas db: %w", err)
	}
	defer func() { _ = db.Close() }()

	return StorePatternAtlasDB(db, atlas)
}

func StorePatternAtlasDB(db *sql.DB, atlas PatternAtlas) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin atlas tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{
		`DELETE FROM pattern_atlas_lints`,
		`DELETE FROM pattern_atlas_cards`,
		`DELETE FROM pattern_atlas_nodes`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("clear atlas table: %w", err)
		}
	}

	nodeInsert, err := tx.Prepare(`
		INSERT INTO pattern_atlas_nodes (
			node_id, pattern_id, heading, level, start_line, end_line, own_end_line,
			parent_node_id, path, body, content_hash, source_ref, fpf_commit
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare atlas node insert: %w", err)
	}
	defer func() { _ = nodeInsert.Close() }()

	for _, node := range atlas.Nodes {
		if _, err := nodeInsert.Exec(
			node.NodeID,
			nullIfEmpty(node.PatternID),
			node.Heading,
			node.Level,
			node.StartLine,
			node.EndLine,
			node.OwnEndLine,
			nullIfEmpty(node.ParentNodeID),
			node.Path,
			node.Body,
			node.ContentHash,
			node.SourceRef,
			node.FPFCommit,
		); err != nil {
			return fmt.Errorf("insert atlas node %s: %w", node.NodeID, err)
		}
	}

	cardInsert, err := tx.Prepare(`
		INSERT INTO pattern_atlas_cards (
			pattern_id, title, card_start_line, card_end_line, root_node_id,
			content_hash, source_ref, fpf_commit
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare atlas card insert: %w", err)
	}
	defer func() { _ = cardInsert.Close() }()

	for _, card := range atlas.Cards {
		if _, err := cardInsert.Exec(
			card.PatternID,
			card.Title,
			card.CardStartLine,
			card.CardEndLine,
			card.RootNodeID,
			card.ContentHash,
			card.SourceRef,
			card.FPFCommit,
		); err != nil {
			return fmt.Errorf("insert atlas card %s: %w", card.PatternID, err)
		}
	}

	lintInsert, err := tx.Prepare(`
		INSERT INTO pattern_atlas_lints (
			line_number, lint_kind, message, raw_line, source_ref, fpf_commit
		) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare atlas lint insert: %w", err)
	}
	defer func() { _ = lintInsert.Close() }()

	for _, lint := range atlas.Lints {
		if _, err := lintInsert.Exec(
			lint.LineNumber,
			lint.LintKind,
			lint.Message,
			lint.RawLine,
			lint.SourceRef,
			lint.FPFCommit,
		); err != nil {
			return fmt.Errorf("insert atlas lint line %d: %w", lint.LineNumber, err)
		}
	}

	return tx.Commit()
}

func GetPatternCard(db *sql.DB, patternID string) (PatternAtlasCardContent, error) {
	patternID = normalizePatternID(patternID)
	if patternID == "" {
		return PatternAtlasCardContent{}, fmt.Errorf("pattern id is required")
	}

	var card PatternAtlasCard
	err := db.QueryRow(`
		SELECT pattern_id, title, card_start_line, card_end_line, root_node_id,
			content_hash, source_ref, fpf_commit
		FROM pattern_atlas_cards
		WHERE pattern_id = ?`, patternID).
		Scan(
			&card.PatternID,
			&card.Title,
			&card.CardStartLine,
			&card.CardEndLine,
			&card.RootNodeID,
			&card.ContentHash,
			&card.SourceRef,
			&card.FPFCommit,
		)
	if err != nil {
		return PatternAtlasCardContent{}, err
	}

	body, nodeCount, err := loadPatternAtlasBodyRange(db, card.CardStartLine, card.CardEndLine)
	if err != nil {
		return PatternAtlasCardContent{}, err
	}

	return PatternAtlasCardContent{
		PatternID:   card.PatternID,
		Title:       card.Title,
		Body:        body,
		StartLine:   card.CardStartLine,
		EndLine:     card.CardEndLine,
		RootNodeID:  card.RootNodeID,
		ContentHash: card.ContentHash,
		SourceRef:   card.SourceRef,
		FPFCommit:   card.FPFCommit,
		NodeCount:   nodeCount,
		BodyKind:    PatternAtlasBodyKindFullCardRange,
	}, nil
}

func GetPatternSubtree(db *sql.DB, nodeID string) (PatternAtlasCardContent, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return PatternAtlasCardContent{}, fmt.Errorf("node id is required")
	}

	var node PatternAtlasNode
	var patternID sql.NullString
	var parentNodeID sql.NullString
	err := db.QueryRow(`
		SELECT node_id, pattern_id, heading, level, start_line, end_line,
			parent_node_id, content_hash, source_ref, fpf_commit
		FROM pattern_atlas_nodes
		WHERE node_id = ?`, nodeID).
		Scan(
			&node.NodeID,
			&patternID,
			&node.Heading,
			&node.Level,
			&node.StartLine,
			&node.EndLine,
			&parentNodeID,
			&node.ContentHash,
			&node.SourceRef,
			&node.FPFCommit,
		)
	if err != nil {
		return PatternAtlasCardContent{}, err
	}
	node.PatternID = patternID.String
	node.ParentNodeID = parentNodeID.String

	body, nodeCount, err := loadPatternAtlasBodyRange(db, node.StartLine, node.EndLine)
	if err != nil {
		return PatternAtlasCardContent{}, err
	}

	return PatternAtlasCardContent{
		PatternID:   node.PatternID,
		Title:       node.Heading,
		Body:        body,
		StartLine:   node.StartLine,
		EndLine:     node.EndLine,
		RootNodeID:  node.NodeID,
		ContentHash: patternAtlasHash(body),
		SourceRef:   node.SourceRef,
		FPFCommit:   node.FPFCommit,
		NodeCount:   nodeCount,
		BodyKind:    PatternAtlasBodyKindFullCardRange,
	}, nil
}

func PatternAtlasCounts(db *sql.DB) (nodes, cards, lints int, err error) {
	if err := db.QueryRow(`SELECT COUNT(*) FROM pattern_atlas_nodes`).Scan(&nodes); err != nil {
		return 0, 0, 0, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pattern_atlas_cards`).Scan(&cards); err != nil {
		return 0, 0, 0, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pattern_atlas_lints`).Scan(&lints); err != nil {
		return 0, 0, 0, err
	}
	return nodes, cards, lints, nil
}

func MissingPatternAtlasCards(db *sql.DB, patternIDs []string) ([]string, error) {
	missing := make([]string, 0)
	for _, patternID := range patternIDs {
		card, err := GetPatternCard(db, patternID)
		if err == sql.ErrNoRows {
			missing = append(missing, normalizePatternID(patternID))
			continue
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(card.Body) == "" || card.NodeCount <= 1 {
			missing = append(missing, normalizePatternID(patternID))
		}
	}
	return missing, nil
}

func PatternAtlasRangeIntegrityErrors(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT node_id, start_line, end_line, own_end_line
		FROM pattern_atlas_nodes
		ORDER BY start_line`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var errs []string
	for rows.Next() {
		var nodeID string
		var startLine int
		var endLine int
		var ownEndLine int
		if err := rows.Scan(&nodeID, &startLine, &endLine, &ownEndLine); err != nil {
			return nil, err
		}
		if startLine <= 0 || ownEndLine < startLine || endLine < ownEndLine {
			errs = append(errs, fmt.Sprintf("%s has invalid range start=%d own_end=%d end=%d", nodeID, startLine, ownEndLine, endLine))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	cardRows, err := db.Query(`
		SELECT pattern_id, card_start_line, card_end_line
		FROM pattern_atlas_cards
		ORDER BY card_start_line`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cardRows.Close() }()

	for cardRows.Next() {
		var patternID string
		var startLine int
		var endLine int
		if err := cardRows.Scan(&patternID, &startLine, &endLine); err != nil {
			return nil, err
		}
		if startLine <= 0 || endLine < startLine {
			errs = append(errs, fmt.Sprintf("%s has invalid card range start=%d end=%d", patternID, startLine, endLine))
		}
	}
	return errs, cardRows.Err()
}

func PatternAtlasHashIntegrityErrors(db *sql.DB) ([]string, error) {
	nodeRows, err := db.Query(`
		SELECT node_id, body, content_hash
		FROM pattern_atlas_nodes
		ORDER BY start_line`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = nodeRows.Close() }()

	errs := make([]string, 0)
	for nodeRows.Next() {
		var nodeID string
		var body string
		var contentHash string
		if err := nodeRows.Scan(&nodeID, &body, &contentHash); err != nil {
			return nil, err
		}
		if got := patternAtlasHash(body); got != contentHash {
			errs = append(errs, fmt.Sprintf("%s node hash mismatch", nodeID))
		}
	}
	if err := nodeRows.Err(); err != nil {
		return nil, err
	}

	cardRows, err := db.Query(`
		SELECT pattern_id, card_start_line, card_end_line, content_hash
		FROM pattern_atlas_cards
		ORDER BY card_start_line`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cardRows.Close() }()

	for cardRows.Next() {
		var patternID string
		var startLine int
		var endLine int
		var contentHash string
		if err := cardRows.Scan(&patternID, &startLine, &endLine, &contentHash); err != nil {
			return nil, err
		}
		body, _, err := loadPatternAtlasBodyRange(db, startLine, endLine)
		if err != nil {
			return nil, err
		}
		if got := patternAtlasHash(body); got != contentHash {
			errs = append(errs, fmt.Sprintf("%s card hash mismatch", patternID))
		}
	}
	return errs, cardRows.Err()
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

func loadPatternAtlasBodyRange(db *sql.DB, startLine, endLine int) (string, int, error) {
	rows, err := db.Query(`
		SELECT body
		FROM pattern_atlas_nodes
		WHERE start_line >= ? AND start_line <= ?
		ORDER BY start_line`, startLine, endLine)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = rows.Close() }()

	parts := make([]string, 0)
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return "", 0, err
		}
		if strings.TrimSpace(body) != "" {
			parts = append(parts, body)
		}
	}
	if err := rows.Err(); err != nil {
		return "", 0, err
	}
	return strings.Join(parts, "\n"), len(parts), nil
}
