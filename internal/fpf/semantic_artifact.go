package fpf

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	SemanticArtifactVersion       = "1"
	defaultSemanticSnippetLimit   = 280
	defaultSemanticBodyPreviewLen = 1200
)

// SemanticArtifact stores an optional embedding index outside the supported
// deterministic SQLite retriever.
type SemanticArtifact struct {
	Version         string                     `json:"version"`
	Provider        string                     `json:"provider"`
	Model           string                     `json:"model"`
	Dimensions      int                        `json:"dimensions"`
	BuiltAt         string                     `json:"built_at"`
	SpecPath        string                     `json:"spec_path,omitempty"`
	IndexCommit     string                     `json:"index_commit,omitempty"`
	IndexedSections string                     `json:"indexed_sections,omitempty"`
	SchemaVersion   string                     `json:"schema_version,omitempty"`
	Documents       []SemanticArtifactDocument `json:"documents"`
	Routes          []SemanticArtifactRoute    `json:"routes"`
}

// SemanticArtifactDocument is the embedded section vector payload.
type SemanticArtifactDocument struct {
	PatternID string    `json:"pattern_id"`
	Vector    []float32 `json:"vector"`
}

// SemanticArtifactRoute is the embedded route vector payload.
type SemanticArtifactRoute struct {
	RouteID string    `json:"route_id"`
	Vector  []float32 `json:"vector"`
}

type semanticDocument struct {
	PatternID   string
	Heading     string
	Summary     string
	Snippet     string
	BodyPreview string
	Keywords    []string
	Queries     []string
	Aliases     []string
}

// BuildSemanticArtifact creates or overwrites the optional semantic embedding
// artifact at the requested path.
func BuildSemanticArtifact(
	ctx context.Context,
	db *sql.DB,
	embedder SemanticEmbedder,
	path string,
) error {
	artifact, err := BuildSemanticArtifactData(ctx, db, embedder)
	if err != nil {
		return err
	}

	return WriteSemanticArtifact(path, artifact)
}

// BuildSemanticArtifactData constructs the embedding payload without touching
// the filesystem, which keeps tests pure.
func BuildSemanticArtifactData(
	ctx context.Context,
	db *sql.DB,
	embedder SemanticEmbedder,
) (SemanticArtifact, error) {
	if embedder == nil {
		return SemanticArtifact{}, fmt.Errorf("semantic embedder is required")
	}

	documents, err := loadSemanticDocuments(db)
	if err != nil {
		return SemanticArtifact{}, err
	}
	routes, err := loadIndexedRoutes(db)
	if err != nil {
		return SemanticArtifact{}, err
	}

	documentTexts := make([]string, 0, len(documents))
	for _, document := range documents {
		documentTexts = append(documentTexts, buildSemanticDocumentText(document))
	}

	routeTexts := make([]string, 0, len(routes))
	for _, route := range routes {
		routeTexts = append(routeTexts, buildSemanticRouteText(route))
	}

	documentVectors, err := embedder.EmbedTexts(ctx, documentTexts)
	if err != nil {
		return SemanticArtifact{}, fmt.Errorf("embed sections: %w", err)
	}
	routeVectors, err := embedder.EmbedTexts(ctx, routeTexts)
	if err != nil {
		return SemanticArtifact{}, fmt.Errorf("embed routes: %w", err)
	}
	if len(documentVectors) != len(documents) {
		return SemanticArtifact{}, fmt.Errorf("embed sections: got %d vectors for %d documents", len(documentVectors), len(documents))
	}
	if len(routeVectors) != len(routes) {
		return SemanticArtifact{}, fmt.Errorf("embed routes: got %d vectors for %d routes", len(routeVectors), len(routes))
	}

	indexInfo, err := GetSpecIndexInfo(db)
	if err != nil {
		return SemanticArtifact{}, fmt.Errorf("read spec metadata: %w", err)
	}
	descriptor := embedder.Descriptor()
	artifact := SemanticArtifact{
		Version:         SemanticArtifactVersion,
		Provider:        descriptor.Provider,
		Model:           descriptor.Model,
		Dimensions:      descriptor.Dimensions,
		BuiltAt:         time.Now().UTC().Format(time.RFC3339),
		SpecPath:        indexInfo.SpecPath,
		IndexCommit:     indexInfo.Commit,
		IndexedSections: indexInfo.IndexedSections,
		SchemaVersion:   indexInfo.SchemaVersion,
		Documents:       make([]SemanticArtifactDocument, 0, len(documents)),
		Routes:          make([]SemanticArtifactRoute, 0, len(routes)),
	}

	for index, document := range documents {
		artifact.Documents = append(artifact.Documents, SemanticArtifactDocument{
			PatternID: document.PatternID,
			Vector:    normalizeEmbeddingVector(documentVectors[index]),
		})
	}
	for index, route := range routes {
		artifact.Routes = append(artifact.Routes, SemanticArtifactRoute{
			RouteID: route.ID,
			Vector:  normalizeEmbeddingVector(routeVectors[index]),
		})
	}

	sort.Slice(artifact.Documents, func(i, j int) bool {
		return artifact.Documents[i].PatternID < artifact.Documents[j].PatternID
	})
	sort.Slice(artifact.Routes, func(i, j int) bool {
		return artifact.Routes[i].RouteID < artifact.Routes[j].RouteID
	})

	return artifact, nil
}

// WriteSemanticArtifact persists the semantic artifact as json or json.gz.
func WriteSemanticArtifact(path string, artifact SemanticArtifact) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("semantic artifact path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create semantic artifact dir: %w", err)
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create semantic artifact: %w", err)
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return fmt.Errorf("chmod semantic artifact temp file: %w", err)
	}

	tempPath := file.Name()
	writer := fileWriterForSemanticArtifact(path, file)
	cleanup := true
	defer func() {
		if !cleanup {
			return
		}
		_ = os.Remove(tempPath)
	}()

	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(artifact); err != nil {
		_ = writer.Close()
		return fmt.Errorf("encode semantic artifact: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize semantic artifact: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("install semantic artifact: %w", err)
	}

	cleanup = false
	return nil
}

// LoadSemanticArtifact reads a json or json.gz semantic artifact.
func LoadSemanticArtifact(path string) (SemanticArtifact, error) {
	if strings.TrimSpace(path) == "" {
		return SemanticArtifact{}, fmt.Errorf("semantic artifact path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SemanticArtifact{}, fmt.Errorf(
				"semantic artifact not found at %q: build it with `haft fpf semantic-index --artifact %q`",
				path,
				path,
			)
		}
		return SemanticArtifact{}, fmt.Errorf("open semantic artifact: %w", err)
	}

	reader, err := fileReaderForSemanticArtifact(path, file)
	if err != nil {
		_ = file.Close()
		return SemanticArtifact{}, err
	}
	defer func() { _ = reader.Close() }()

	artifact := SemanticArtifact{}
	if err := json.NewDecoder(reader).Decode(&artifact); err != nil {
		return SemanticArtifact{}, fmt.Errorf("decode semantic artifact: %w", err)
	}

	return artifact, nil
}

// DefaultSemanticArtifactPath keeps the optional artifact under ignored cache
// storage instead of mixing it into the repo.
func DefaultSemanticArtifactPath(model string, dimensions int) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}

	slug := slugSemanticModel(model)
	fileName := fmt.Sprintf("fpf-semantic-%s-%d.json.gz", slug, dimensions)
	return filepath.Join(home, ".cache", "haft", fileName), nil
}

func buildSemanticDocumentText(document semanticDocument) string {
	parts := []string{
		document.PatternID,
		document.Heading,
		document.Summary,
		document.BodyPreview,
	}

	if len(document.Aliases) > 0 {
		parts = append(parts, strings.Join(document.Aliases, ", "))
	}
	if len(document.Keywords) > 0 {
		parts = append(parts, strings.Join(document.Keywords, ", "))
	}
	if len(document.Queries) > 0 {
		parts = append(parts, strings.Join(document.Queries, " "))
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func buildSemanticRouteText(route Route) string {
	parts := []string{route.Title, route.Description}
	if len(route.Matchers) > 0 {
		parts = append(parts, strings.Join(route.Matchers, " "))
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
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
	`, defaultSemanticSnippetLimit, defaultSemanticBodyPreviewLen)
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

//nolint:unused // exercised by package tests
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

func validateSemanticArtifact(
	db *sql.DB,
	artifact SemanticArtifact,
	descriptor SemanticEmbedderDescriptor,
) error {
	if artifact.Version != SemanticArtifactVersion {
		return fmt.Errorf(
			"semantic artifact version mismatch: got %q want %q",
			artifact.Version,
			SemanticArtifactVersion,
		)
	}

	if artifact.Provider != descriptor.Provider ||
		artifact.Model != descriptor.Model ||
		artifact.Dimensions != descriptor.Dimensions {
		return fmt.Errorf(
			"semantic artifact model mismatch: artifact=%s/%s/%d query=%s/%s/%d",
			artifact.Provider,
			artifact.Model,
			artifact.Dimensions,
			descriptor.Provider,
			descriptor.Model,
			descriptor.Dimensions,
		)
	}

	indexInfo, err := GetSpecIndexInfo(db)
	if err != nil {
		return fmt.Errorf("read current spec metadata: %w", err)
	}

	if artifact.SchemaVersion != "" &&
		indexInfo.SchemaVersion != "" &&
		artifact.SchemaVersion != indexInfo.SchemaVersion {
		return fmt.Errorf(
			"semantic artifact schema mismatch: artifact=%s current=%s",
			artifact.SchemaVersion,
			indexInfo.SchemaVersion,
		)
	}
	if artifact.IndexCommit != "" &&
		indexInfo.Commit != "" &&
		artifact.IndexCommit != indexInfo.Commit {
		return fmt.Errorf(
			"semantic artifact commit mismatch: artifact=%s current=%s",
			artifact.IndexCommit,
			indexInfo.Commit,
		)
	}
	if artifact.IndexedSections != "" &&
		indexInfo.IndexedSections != "" &&
		artifact.IndexedSections != indexInfo.IndexedSections {
		return fmt.Errorf(
			"semantic artifact section-count mismatch: artifact=%s current=%s",
			artifact.IndexedSections,
			indexInfo.IndexedSections,
		)
	}

	return nil
}

type semanticReadCloser struct {
	reader *gzip.Reader
	file   *os.File
}

func (reader semanticReadCloser) Read(p []byte) (int, error) {
	return reader.reader.Read(p)
}

func (reader semanticReadCloser) Close() error {
	closeErr := reader.reader.Close()
	fileErr := reader.file.Close()
	if closeErr != nil {
		return closeErr
	}
	return fileErr
}

type semanticWriteCloser struct {
	writer *gzip.Writer
	file   *os.File
}

func (writer semanticWriteCloser) Write(p []byte) (int, error) {
	return writer.writer.Write(p)
}

func (writer semanticWriteCloser) Close() error {
	closeErr := writer.writer.Close()
	syncErr := writer.file.Sync()
	fileErr := writer.file.Close()
	if closeErr != nil {
		return closeErr
	}
	if syncErr != nil {
		return syncErr
	}
	return fileErr
}

type semanticPlainWriteCloser struct {
	file *os.File
}

func (writer semanticPlainWriteCloser) Write(p []byte) (int, error) {
	return writer.file.Write(p)
}

func (writer semanticPlainWriteCloser) Close() error {
	syncErr := writer.file.Sync()
	closeErr := writer.file.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func fileReaderForSemanticArtifact(path string, file *os.File) (interface {
	Read(p []byte) (int, error)
	Close() error
}, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".gz") {
		return file, nil
	}

	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open semantic artifact gzip: %w", err)
	}

	return semanticReadCloser{
		reader: reader,
		file:   file,
	}, nil
}

func fileWriterForSemanticArtifact(path string, file *os.File) io.WriteCloser {
	if !strings.HasSuffix(strings.ToLower(path), ".gz") {
		return semanticPlainWriteCloser{file: file}
	}

	return semanticWriteCloser{
		writer: gzip.NewWriter(file),
		file:   file,
	}
}

func slugSemanticModel(model string) string {
	slug := strings.ToLower(strings.TrimSpace(model))
	slug = strings.NewReplacer("/", "-", ":", "-", " ", "-").Replace(slug)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "unknown-model"
	}
	return slug
}
