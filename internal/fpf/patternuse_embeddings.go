package fpf

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

const (
	PatternUseRouteDocumentKindSynopsis        = "synopsis"
	PatternUseRouteDocumentKindPositiveExample = "positive_example"
	PatternUseRouteDocumentKindNegativeExample = "negative_example"
	patternUseRouteEmbeddingBatch              = 16
)

type PatternUseRouteEmbeddingDocument struct {
	RouteID      string
	DocumentID   string
	DocumentKind string
	Text         string
	ContentHash  string
}

type PatternUseRouteEmbeddingRow struct {
	RouteID      string
	DocumentID   string
	DocumentKind string
	ContentHash  string
	Vector       []float32
}

type PatternUseIntentEmbeddingRow struct {
	LaneID       PatternUseIntentLane
	DocumentID   string
	DocumentKind string
	ContentHash  string
	Vector       []float32
}

func PatternUseRouteEmbeddingDocuments(routes []PatternUseRouteCard) []PatternUseRouteEmbeddingDocument {
	documents := []PatternUseRouteEmbeddingDocument{}
	for _, route := range clonePatternUseRouteCards(routes) {
		if route.SupportLevel != PatternUseSupportImplementedSubstrate {
			continue
		}

		synopsis := patternUseRouteSynopsisDocument(route)
		documents = append(documents, patternUseRouteEmbeddingDocument(
			route.ID,
			route.ID+":synopsis",
			PatternUseRouteDocumentKindSynopsis,
			synopsis,
		))

		for index, example := range route.PositiveExamples {
			documentID := fmt.Sprintf("%s:positive:%02d", route.ID, index+1)
			documents = append(documents, patternUseRouteEmbeddingDocument(
				route.ID,
				documentID,
				PatternUseRouteDocumentKindPositiveExample,
				example,
			))
		}

		for index, example := range route.NegativeExamples {
			documentID := fmt.Sprintf("%s:negative:%02d", route.ID, index+1)
			documents = append(documents, patternUseRouteEmbeddingDocument(
				route.ID,
				documentID,
				PatternUseRouteDocumentKindNegativeExample,
				example,
			))
		}
	}
	return documents
}

func BakePatternUseRouteEmbeddings(
	ctx context.Context,
	dbPath string,
	embedder SemanticEmbedder,
	routes []PatternUseRouteCard,
) (int, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, fmt.Errorf("open index for pattern-use route bake: %w", err)
	}
	defer func() { _ = db.Close() }()

	documents := PatternUseRouteEmbeddingDocuments(routes)
	if len(documents) == 0 {
		return 0, nil
	}

	descriptor := embedder.Descriptor()
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin pattern-use route bake tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM pattern_use_route_embeddings WHERE provider=? AND model=? AND dim=?`,
		descriptor.Provider,
		descriptor.Model,
		descriptor.Dimensions,
	); err != nil {
		return 0, fmt.Errorf("clear existing pattern-use route embeddings: %w", err)
	}

	insert, err := tx.Prepare(`
		INSERT INTO pattern_use_route_embeddings
			(route_id, document_id, document_kind, provider, model, dim, content_hash, vector)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare pattern-use route bake insert: %w", err)
	}
	defer func() { _ = insert.Close() }()

	for start := 0; start < len(documents); start += patternUseRouteEmbeddingBatch {
		end := min(start+patternUseRouteEmbeddingBatch, len(documents))
		batch := documents[start:end]
		texts := make([]string, len(batch))
		for index, document := range batch {
			texts[index] = document.Text
		}

		vectors, err := embedder.EmbedTexts(ctx, texts)
		if err != nil {
			return 0, fmt.Errorf("embed pattern-use route documents [%d:%d]: %w", start, end, err)
		}
		fmt.Fprintf(os.Stderr, "  baking PatternUse route vectors: %d/%d\n", end, len(documents))
		if len(vectors) != len(batch) {
			return 0, fmt.Errorf("embed pattern-use route documents: got %d vectors for %d texts", len(vectors), len(batch))
		}

		for index, document := range batch {
			normalized := normalizeSpecVector(vectors[index])
			_, err := insert.Exec(
				document.RouteID,
				document.DocumentID,
				document.DocumentKind,
				descriptor.Provider,
				descriptor.Model,
				descriptor.Dimensions,
				document.ContentHash,
				encodeSpecVector(normalized),
			)
			if err != nil {
				return 0, fmt.Errorf("store pattern-use route document %s vector: %w", document.DocumentID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(documents), nil
}

func BakePatternUseIntentEmbeddings(
	ctx context.Context,
	dbPath string,
	embedder SemanticEmbedder,
	cards []PatternUseIntentLaneCard,
) (int, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, fmt.Errorf("open index for pattern-use intent bake: %w", err)
	}
	defer func() { _ = db.Close() }()

	documents := PatternUseIntentEmbeddingDocuments(cards)
	if len(documents) == 0 {
		return 0, nil
	}

	descriptor := embedder.Descriptor()
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin pattern-use intent bake tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM pattern_use_intent_embeddings WHERE provider=? AND model=? AND dim=?`,
		descriptor.Provider,
		descriptor.Model,
		descriptor.Dimensions,
	); err != nil {
		return 0, fmt.Errorf("clear existing pattern-use intent embeddings: %w", err)
	}

	insert, err := tx.Prepare(`
		INSERT INTO pattern_use_intent_embeddings
			(lane_id, document_id, document_kind, provider, model, dim, content_hash, vector)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare pattern-use intent bake insert: %w", err)
	}
	defer func() { _ = insert.Close() }()

	for start := 0; start < len(documents); start += patternUseRouteEmbeddingBatch {
		end := min(start+patternUseRouteEmbeddingBatch, len(documents))
		batch := documents[start:end]
		texts := make([]string, len(batch))
		for index, document := range batch {
			texts[index] = document.Text
		}

		vectors, err := embedder.EmbedTexts(ctx, texts)
		if err != nil {
			return 0, fmt.Errorf("embed pattern-use intent documents [%d:%d]: %w", start, end, err)
		}
		fmt.Fprintf(os.Stderr, "  baking PatternUse intent vectors: %d/%d\n", end, len(documents))
		if len(vectors) != len(batch) {
			return 0, fmt.Errorf("embed pattern-use intent documents: got %d vectors for %d texts", len(vectors), len(batch))
		}

		for index, document := range batch {
			normalized := normalizeSpecVector(vectors[index])
			_, err := insert.Exec(
				document.LaneID,
				document.DocumentID,
				document.DocumentKind,
				descriptor.Provider,
				descriptor.Model,
				descriptor.Dimensions,
				document.ContentHash,
				encodeSpecVector(normalized),
			)
			if err != nil {
				return 0, fmt.Errorf("store pattern-use intent document %s vector: %w", document.DocumentID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(documents), nil
}

func LoadPatternUseRouteEmbeddings(
	db *sql.DB,
	provider string,
	model string,
	dim int,
) ([]PatternUseRouteEmbeddingRow, error) {
	rows, err := db.Query(`
		SELECT route_id, document_id, document_kind, content_hash, vector
		FROM pattern_use_route_embeddings
		WHERE provider=? AND model=? AND dim=?
		ORDER BY route_id, document_id`,
		provider,
		model,
		dim,
	)
	if err != nil {
		return nil, fmt.Errorf("load pattern-use route embeddings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []PatternUseRouteEmbeddingRow{}
	for rows.Next() {
		var row PatternUseRouteEmbeddingRow
		var vector []byte
		err := rows.Scan(&row.RouteID, &row.DocumentID, &row.DocumentKind, &row.ContentHash, &vector)
		if err != nil {
			return nil, err
		}
		row.Vector = decodeSpecVector(vector)
		if row.Vector == nil {
			continue
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func LoadPatternUseIntentEmbeddings(
	db *sql.DB,
	provider string,
	model string,
	dim int,
) ([]PatternUseIntentEmbeddingRow, error) {
	rows, err := db.Query(`
		SELECT lane_id, document_id, document_kind, content_hash, vector
		FROM pattern_use_intent_embeddings
		WHERE provider=? AND model=? AND dim=?
		ORDER BY lane_id, document_id`,
		provider,
		model,
		dim,
	)
	if err != nil {
		return nil, fmt.Errorf("load pattern-use intent embeddings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []PatternUseIntentEmbeddingRow{}
	for rows.Next() {
		var row PatternUseIntentEmbeddingRow
		var laneID string
		var vector []byte
		err := rows.Scan(&laneID, &row.DocumentID, &row.DocumentKind, &row.ContentHash, &vector)
		if err != nil {
			return nil, err
		}
		row.LaneID = PatternUseIntentLane(strings.TrimSpace(laneID))
		row.Vector = decodeSpecVector(vector)
		if row.Vector == nil {
			continue
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func CountPatternUseRouteEmbeddingsForContract(
	db *sql.DB,
	provider string,
	model string,
	dim int,
) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM pattern_use_route_embeddings WHERE provider=? AND model=? AND dim=?`,
		provider,
		model,
		dim,
	).Scan(&count)
	return count, err
}

func CountPatternUseIntentEmbeddingsForContract(
	db *sql.DB,
	provider string,
	model string,
	dim int,
) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM pattern_use_intent_embeddings WHERE provider=? AND model=? AND dim=?`,
		provider,
		model,
		dim,
	).Scan(&count)
	return count, err
}

func MissingPatternUseRouteEmbeddingDocuments(
	db *sql.DB,
	provider string,
	model string,
	dim int,
	documents []PatternUseRouteEmbeddingDocument,
) ([]string, error) {
	missing := []string{}
	for _, document := range documents {
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM pattern_use_route_embeddings
			WHERE route_id=? AND document_id=? AND provider=? AND model=? AND dim=? AND content_hash=?`,
			document.RouteID,
			document.DocumentID,
			provider,
			model,
			dim,
			document.ContentHash,
		).Scan(&count)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			missing = append(missing, document.DocumentID)
		}
	}
	return missing, nil
}

func MissingPatternUseIntentEmbeddingDocuments(
	db *sql.DB,
	provider string,
	model string,
	dim int,
	documents []PatternUseIntentEmbeddingDocument,
) ([]string, error) {
	missing := []string{}
	for _, document := range documents {
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM pattern_use_intent_embeddings
			WHERE lane_id=? AND document_id=? AND provider=? AND model=? AND dim=? AND content_hash=?`,
			document.LaneID,
			document.DocumentID,
			provider,
			model,
			dim,
			document.ContentHash,
		).Scan(&count)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			missing = append(missing, document.DocumentID)
		}
	}
	return missing, nil
}

func PatternUseRouteEmbeddingContract(db *sql.DB) (provider, model string, dim, count int, err error) {
	row := db.QueryRow(`
		SELECT provider, model, dim, COUNT(*)
		FROM pattern_use_route_embeddings
		GROUP BY provider, model, dim
		ORDER BY COUNT(*) DESC
		LIMIT 1`)
	err = row.Scan(&provider, &model, &dim, &count)
	if err == sql.ErrNoRows {
		return "", "", 0, 0, nil
	}
	return provider, model, dim, count, err
}

func PatternUseIntentEmbeddingContract(db *sql.DB) (provider, model string, dim, count int, err error) {
	row := db.QueryRow(`
		SELECT provider, model, dim, COUNT(*)
		FROM pattern_use_intent_embeddings
		GROUP BY provider, model, dim
		ORDER BY COUNT(*) DESC
		LIMIT 1`)
	err = row.Scan(&provider, &model, &dim, &count)
	if err == sql.ErrNoRows {
		return "", "", 0, 0, nil
	}
	return provider, model, dim, count, err
}

func patternUseRouteEmbeddingDocument(
	routeID string,
	documentID string,
	documentKind string,
	text string,
) PatternUseRouteEmbeddingDocument {
	canonicalText := strings.TrimSpace(text)
	return PatternUseRouteEmbeddingDocument{
		RouteID:      strings.TrimSpace(routeID),
		DocumentID:   strings.TrimSpace(documentID),
		DocumentKind: strings.TrimSpace(documentKind),
		Text:         canonicalText,
		ContentHash:  specContentHash(canonicalText),
	}
}

func patternUseRouteSynopsisDocument(route PatternUseRouteCard) string {
	parts := []string{
		"route_id: " + route.ID,
		"trigger: " + route.SemanticTrigger,
		"recommended_pattern: " + route.RecommendedPatternUse.PatternRef + " " + route.RecommendedPatternUse.Title,
		"reason: " + route.ReasonForRecommendation,
		"output_shape: " + patternUseOutputShapeText(route.RequiredOutputShape),
		"wrong_boundary: " + patternUseWrongBoundarySummary(route.WrongPatternBoundary),
		"blocked_stronger_use: " + patternUseBlockedSummary(route.BlockedStrongerUse),
		"suggested_surface: " + firstNonEmptyPatternUseString(route.SuggestedHaftSurface, suggestedHaftSurfaceForPatternUseRoute(route.ID)),
		"method_refs: " + strings.Join(route.SuggestedMethodRefs, " "),
		"governing_patterns: " + strings.Join(route.NextGoverningPatternRefs, " "),
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func patternUseOutputShapeText(shape RequiredOutputShape) string {
	parts := append([]string{shape.CarrierKind}, shape.RequiredSections...)
	return strings.Join(parts, " ")
}

func patternUseWrongBoundarySummary(boundaries []WrongPatternBoundary) string {
	parts := []string{}
	for _, boundary := range boundaries {
		parts = append(parts, boundary.TemptingPatternOrMove, boundary.WhyWrongNow)
	}
	return strings.Join(parts, " ")
}

func patternUseBlockedSummary(blocked []BlockedStrongerUse) string {
	parts := []string{}
	for _, item := range blocked {
		parts = append(parts, item.BlockedUse, item.UnblockCondition)
	}
	return strings.Join(parts, " ")
}
