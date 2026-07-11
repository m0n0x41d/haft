package artifact

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
	"github.com/m0n0x41d/haft/internal/textsearch"
)

// Store handles artifact persistence in SQLite.
// Implements ArtifactStore interface.
type Store struct {
	db       *sql.DB
	colCache map[string]bool // cached tableHasColumn results: "table.column" → exists
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type sqlQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Compile-time check: Store must implement ArtifactStore.
var _ ArtifactStore = (*Store)(nil)

// NewStore creates a new artifact store using an existing DB connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, colCache: make(map[string]bool)}
}

// DB returns the underlying database connection.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Create inserts a new artifact into the database.
func (s *Store) Create(ctx context.Context, a *Artifact) error {
	now := time.Now().UTC()
	if a.Meta.CreatedAt.IsZero() {
		a.Meta.CreatedAt = now
	}
	a.Meta.UpdatedAt = now
	if a.Meta.Version == 0 {
		a.Meta.Version = 1
	}
	if a.Meta.Status == "" {
		a.Meta.Status = StatusActive
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for artifact %s: %w", a.Meta.ID, err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO artifacts (id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at, search_keywords, structured_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Meta.ID, string(a.Meta.Kind), a.Meta.Version, string(a.Meta.Status),
		a.Meta.Context, string(a.Meta.Mode), a.Meta.Title, a.Body,
		a.Meta.ValidUntil,
		a.Meta.CreatedAt.Format(time.RFC3339),
		a.Meta.UpdatedAt.Format(time.RFC3339),
		a.SearchKeywords,
		a.StructuredData,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("artifact %s already exists", a.Meta.ID)
		}
		return fmt.Errorf("insert artifact %s: %w", a.Meta.ID, err)
	}

	for _, link := range a.Meta.Links {
		_, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO artifact_links (source_id, target_id, link_type, created_at)
			VALUES (?, ?, ?, ?)`, a.Meta.ID, link.Ref, link.Type, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("insert link %s→%s: %w", a.Meta.ID, link.Ref, err)
		}
	}

	return tx.Commit()
}

// Get retrieves an artifact by ID.
func (s *Store) Get(ctx context.Context, id string) (*Artifact, error) {
	var a Artifact
	var kind, status, mode, validUntil, context_, createdAt, updatedAt string
	var searchKeywords, structuredData sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at, COALESCE(search_keywords, ''), COALESCE(structured_data, '')
		FROM artifacts WHERE id = ?`, id,
	).Scan(
		&a.Meta.ID, &kind, &a.Meta.Version, &status, &context_, &mode,
		&a.Meta.Title, &a.Body, &validUntil, &createdAt, &updatedAt, &searchKeywords, &structuredData,
	)
	if err != nil {
		return nil, fmt.Errorf("get artifact %s: %w", id, err)
	}
	if k, err := ParseKind(kind); err == nil {
		a.Meta.Kind = k
	} else {
		a.Meta.Kind = Kind(kind) // preserve unknown kinds from older schema
	}
	if st, err := ParseStatus(status); err == nil {
		a.Meta.Status = st
	} else {
		a.Meta.Status = Status(status)
	}
	if m, err := ParseMode(mode); err == nil {
		a.Meta.Mode = m
	} else {
		a.Meta.Mode = Mode(mode)
	}
	a.Meta.Context = context_
	a.Meta.ValidUntil = validUntil
	a.Meta.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.Meta.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	a.SearchKeywords = searchKeywords.String
	a.StructuredData = structuredData.String

	links, err := s.GetLinks(ctx, id)
	if err == nil {
		a.Meta.Links = links
	}

	return &a, nil
}

// Update modifies an existing artifact.
func (s *Store) Update(ctx context.Context, a *Artifact) error {
	return s.updateArtifactWithExec(ctx, s.db, a)
}

func (s *Store) updateArtifactWithExec(ctx context.Context, execer sqlExecer, a *Artifact) error {
	a.Meta.UpdatedAt = time.Now().UTC()
	a.Meta.Version++

	result, err := execer.ExecContext(ctx, `
		UPDATE artifacts SET kind=?, version=?, status=?, context=?, mode=?, title=?, content=?, valid_until=?, updated_at=?, search_keywords=?, structured_data=?
		WHERE id=?`,
		string(a.Meta.Kind), a.Meta.Version, string(a.Meta.Status),
		a.Meta.Context, string(a.Meta.Mode), a.Meta.Title, a.Body,
		a.Meta.ValidUntil, a.Meta.UpdatedAt.Format(time.RFC3339),
		a.SearchKeywords, a.StructuredData,
		a.Meta.ID,
	)
	if err != nil {
		return fmt.Errorf("update artifact %s: %w", a.Meta.ID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("artifact %s not found", a.Meta.ID)
	}
	return nil
}

// ListByKind returns artifacts of a given kind, ordered by creation time descending.
// If kind is empty, returns all artifacts regardless of kind.
func (s *Store) ListByKind(ctx context.Context, kind Kind, limit int) ([]*Artifact, error) {
	var rows *sql.Rows
	var err error
	if kind == "" {
		if limit > 0 {
			rows, err = s.db.QueryContext(ctx, `
				SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
				FROM artifacts ORDER BY created_at DESC LIMIT ?`, limit)
		} else {
			rows, err = s.db.QueryContext(ctx, `
				SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
				FROM artifacts ORDER BY created_at DESC`)
		}
	} else if limit > 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts WHERE kind = ? ORDER BY created_at DESC LIMIT ?`,
			string(kind), limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts WHERE kind = ? ORDER BY created_at DESC`,
			string(kind))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// ListActiveByKind returns non-deprecated, non-superseded artifacts of the given kind.
func (s *Store) ListActiveByKind(ctx context.Context, kind Kind, limit int) ([]*Artifact, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if limit > 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts WHERE kind = ? AND status = 'active'
			ORDER BY created_at DESC LIMIT ?`,
			string(kind), limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts WHERE kind = ? AND status = 'active'
			ORDER BY created_at DESC`,
			string(kind))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// ListByContext returns artifacts for a given context, ordered by creation time.
func (s *Store) ListByContext(ctx context.Context, contextName string) ([]*Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
		FROM artifacts WHERE context = ? ORDER BY created_at DESC`,
		contextName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// ListActive returns active (non-deprecated, non-superseded) artifacts.
func (s *Store) ListActive(ctx context.Context, limit int) ([]*Artifact, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if limit > 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts WHERE status NOT IN ('superseded', 'deprecated') ORDER BY updated_at DESC LIMIT ?`,
			limit,
		)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts WHERE status NOT IN ('superseded', 'deprecated') ORDER BY updated_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// Search performs exact lookup for artifact IDs and FTS5 full-text search for
// discovery queries.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]*Artifact, error) {
	if limit <= 0 {
		limit = 20
	}
	exactID := strings.TrimSpace(query)
	if IsArtifactID(exactID) {
		item, err := s.Get(ctx, exactID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []*Artifact{item}, nil
	}

	// Phase-1 term prep (dec-20260604-3aaad199): split compound identifiers
	// (getUserName -> get/user/name + getusername) and drop stop-words before
	// FTS. Stems are OFF here — artifacts_fts is porter-tokenized, so the index
	// already stems both sides; double-stemming would only add noise. Terms
	// yields alnum-only lower-case tokens, so they are inherently FTS5-operator
	// safe (no manual special-char stripping needed). A `kind:` qualifier narrows
	// the reasoning lane to one or more artifact kinds.
	//
	var ftsTerms []string
	pq := textsearch.ParseQuery(query)
	kindFilter := matchArtifactKinds(pq.Kinds)
	for _, term := range textsearch.Terms(pq.Text, textsearch.Options{Stems: false}) {
		ftsTerms = append(ftsTerms, fmt.Sprintf(`"%s"*`, term))
	}

	// A kind-only query ("kind:DecisionRecord", no free text) has no FTS terms —
	// list that kind rather than match nothing.
	if len(ftsTerms) == 0 {
		if len(kindFilter) > 0 {
			return s.listByKinds(ctx, kindFilter, limit)
		}
		return nil, nil
	}

	// AND-default: require all terms present (implicit AND = space-join in FTS5).
	results, err := s.searchFTS(ctx, strings.Join(ftsTerms, " "), kindFilter, limit)
	if err != nil {
		return nil, err
	}
	// Fallback to OR if AND returned nothing.
	if len(results) == 0 && len(ftsTerms) > 1 {
		return s.searchFTS(ctx, strings.Join(ftsTerms, " OR "), kindFilter, limit)
	}
	return results, nil
}

func (s *Store) listByKinds(ctx context.Context, kinds []Kind, limit int) ([]*Artifact, error) {
	uniqueKinds := make([]Kind, 0, len(kinds))
	seen := make(map[Kind]struct{}, len(kinds))
	for _, kind := range kinds {
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		uniqueKinds = append(uniqueKinds, kind)
	}
	if len(uniqueKinds) == 0 {
		return nil, nil
	}
	if len(uniqueKinds) == 1 {
		return s.ListByKind(ctx, uniqueKinds[0], limit)
	}

	var sb strings.Builder
	sb.WriteString(`SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
		FROM artifacts WHERE kind IN (`)
	args := make([]any, 0, len(uniqueKinds)+1)
	for i, kind := range uniqueKinds {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('?')
		args = append(args, string(kind))
	}
	sb.WriteString(") ORDER BY created_at DESC")
	if limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// searchFTS runs one MATCH query, optionally constrained to a set of artifact
// kinds (the kind: filter), ranked by the column-weighted bm25 score.
func (s *Store) searchFTS(ctx context.Context, ftsQuery string, kinds []Kind, limit int) ([]*Artifact, error) {
	var sb strings.Builder
	sb.WriteString(`SELECT a.id, a.kind, a.version, a.status, a.context, a.mode, a.title, a.content, a.valid_until, a.created_at, a.updated_at
		FROM artifacts a
		JOIN artifacts_fts f ON a.id = f.id
		WHERE artifacts_fts MATCH ?`)
	args := []any{ftsQuery}
	if len(kinds) > 0 {
		sb.WriteString(" AND a.kind IN (")
		for i, k := range kinds {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteByte('?')
			args = append(args, string(k))
		}
		sb.WriteByte(')')
	}
	sb.WriteString(" ORDER BY bm25(artifacts_fts, 0.0, 10.0, 1.0, 5.0, 3.0) LIMIT ?")
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// matchArtifactKinds maps free-text kind: values to canonical artifact Kinds
// (case-insensitive). Unrecognized values are dropped — a typo'd kind filters
// nothing rather than erroring.
func matchArtifactKinds(raw []string) []Kind {
	if len(raw) == 0 {
		return nil
	}
	known := []Kind{
		KindNote, KindProblemCard, KindSolutionPortfolio, KindDecisionRecord,
		KindWorkCommission, KindEvidencePack, KindRefreshReport,
	}
	var out []Kind
	for _, r := range raw {
		for _, k := range known {
			if strings.EqualFold(r, string(k)) {
				out = append(out, k)
				break
			}
		}
	}
	return out
}

// SearchByAffectedFile finds artifacts linked to a specific file path.
func (s *Store) SearchByAffectedFile(ctx context.Context, filePath string) ([]*Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.kind, a.version, a.status, a.context, a.mode, a.title, a.content, a.valid_until, a.created_at, a.updated_at, a.structured_data
		FROM artifacts a
		JOIN affected_files af ON a.id = af.artifact_id
		WHERE af.file_path = ?
		ORDER BY a.updated_at DESC`,
		filePath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// SearchByAffectedSymbol returns artifacts linked to a specific symbol via
// affected_symbols — the symbol-granular companion to SearchByAffectedFile.
// This is the join that lets an agent exploring a symbol see the decisions /
// problems / variants touching that exact symbol, not just its file. When
// filePath is non-empty, results are scoped to that file so same-named symbols
// in different files don't collide.
func (s *Store) SearchByAffectedSymbol(ctx context.Context, symbolName, filePath string) ([]*Artifact, error) {
	query := `
		SELECT DISTINCT a.id, a.kind, a.version, a.status, a.context, a.mode, a.title, a.content, a.valid_until, a.created_at, a.updated_at, a.structured_data
		FROM artifacts a
		JOIN affected_symbols asym ON a.id = asym.artifact_id
		WHERE asym.symbol_name = ?`
	queryArgs := []any{symbolName}
	if filePath != "" {
		query += ` AND asym.file_path = ?`
		queryArgs = append(queryArgs, filePath)
	}
	query += ` ORDER BY a.updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// SearchByAffectedSymbolAt is the LINE-AWARE companion to SearchByAffectedSymbol:
// it returns only artifacts whose affected_symbols row for this (name, file)
// COVERS the given 1-based line (line within [symbol_line, symbol_end_line]).
// This is the keystone of honest fusion — two same-name methods on different
// receivers occupy disjoint line ranges, so a decision recorded against one
// overload never bleeds onto the other. Rows with no usable end line (legacy or
// 0-valued) cannot be range-matched and are intentionally excluded; the caller
// falls back to the line-blind SearchByAffectedSymbol and LABELS the result as
// file+name granularity rather than presenting false per-symbol precision.
func (s *Store) SearchByAffectedSymbolAt(ctx context.Context, symbolName, filePath string, line int) ([]*Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT a.id, a.kind, a.version, a.status, a.context, a.mode, a.title, a.content, a.valid_until, a.created_at, a.updated_at, a.structured_data
		FROM artifacts a
		JOIN affected_symbols asym ON a.id = asym.artifact_id
		WHERE asym.symbol_name = ?
		  AND asym.file_path = ?
		  AND asym.symbol_end_line >= asym.symbol_line
		  AND ? >= asym.symbol_line
		  AND ? <= asym.symbol_end_line
		ORDER BY a.updated_at DESC`,
		symbolName, filePath, line, line,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// FindStaleDecisions returns decisions past their valid_until or with refresh_due status.
func (s *Store) FindStaleDecisions(ctx context.Context) ([]*Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts
			WHERE kind = ? AND (
				status = 'refresh_due'
				OR valid_until != ''
			)
			ORDER BY valid_until ASC`,
		string(KindDecisionRecord),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artifacts, err := scanArtifacts(rows)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	filtered := filterStaleArtifacts(artifacts, now)
	return filtered, nil
}

// FindStaleArtifacts returns any artifacts (not just decisions) past their valid_until.
// This catches stale ProblemCards, expired characterizations, and old portfolios.
func (s *Store) FindStaleArtifacts(ctx context.Context) ([]*Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `
			SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
			FROM artifacts
			WHERE status NOT IN ('superseded', 'deprecated')
			AND valid_until != ''
			ORDER BY kind, valid_until ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artifacts, err := scanArtifacts(rows)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	filtered := filterStaleArtifacts(artifacts, now)
	return filtered, nil
}

// NextSequence returns the next sequence number for a given kind on a given date.
// Uses MAX(id) to find the highest existing sequence, avoiding TOCTOU race on COUNT.
//
// Deprecated: GenerateID switched to crypto/rand hex suffixes in #63 (6.2).
// All artifact creators now pass 0 to GenerateID and skip this lookup. Kept
// in the interface and Store for one release for external callers; planned
// for removal in 6.3 alongside the seq parameter on GenerateID.
func (s *Store) NextSequence(ctx context.Context, kind Kind) (int, error) {
	prefix := fmt.Sprintf("%s-%s-", kind.IDPrefix(), time.Now().Format("20060102"))
	var maxID sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT MAX(id) FROM artifacts WHERE id LIKE ?`, prefix+"%").Scan(&maxID)
	if err != nil || !maxID.Valid {
		return 1, nil
	}
	// Extract sequence from ID format: kind-YYYYMMDD-NNN
	parts := strings.Split(maxID.String, "-")
	if len(parts) < 3 {
		return 1, nil
	}
	seq := 0
	_, _ = fmt.Sscanf(parts[len(parts)-1], "%d", &seq)
	return seq + 1, nil
}

// --- Links ---

// AddLink creates a link between two artifacts.
func (s *Store) AddLink(ctx context.Context, sourceID, targetID, linkType string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO artifact_links (source_id, target_id, link_type, created_at)
		VALUES (?, ?, ?, ?)`,
		sourceID, targetID, linkType, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetLinks returns all links from a given artifact.
func (s *Store) GetLinks(ctx context.Context, artifactID string) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT target_id, link_type FROM artifact_links WHERE source_id = ?`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.Ref, &l.Type); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// GetBacklinks returns artifacts that link TO a given artifact.
func (s *Store) GetBacklinks(ctx context.Context, artifactID string) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_id, link_type FROM artifact_links WHERE target_id = ?`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.Ref, &l.Type); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// LinkEdge is one artifact_links row — a directed reasoning-graph edge.
type LinkEdge struct {
	Source string
	Target string
	Type   string
}

// AllLinks enumerates every artifact link in one pass — the whole-graph
// enumeration the fused-graph ranker (graphrank, dec-20260604-3aaad199 phase 2)
// uses to build the reasoning-graph adjacency. Stable order = deterministic build.
func (s *Store) AllLinks(ctx context.Context) ([]LinkEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_id, target_id, link_type FROM artifact_links ORDER BY source_id, target_id, link_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkEdge
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.Source, &e.Target, &e.Type); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AffectedFileRef is one affected_files row — an artifact's link to a file path,
// the bridge between the reasoning graph and the code graph.
type AffectedFileRef struct {
	ArtifactID string
	FilePath   string
}

// AllAffectedFiles enumerates every (artifact, file) pair in one pass, stably
// ordered — the artifact<->file half of the fused-graph bridge.
func (s *Store) AllAffectedFiles(ctx context.Context) ([]AffectedFileRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT artifact_id, file_path FROM affected_files ORDER BY artifact_id, file_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AffectedFileRef
	for rows.Next() {
		var r AffectedFileRef
		if err := rows.Scan(&r.ArtifactID, &r.FilePath); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Affected Files ---

// SetAffectedFiles replaces the affected files list for an artifact.
func (s *Store) SetAffectedFiles(ctx context.Context, artifactID string, files []AffectedFile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM affected_files WHERE artifact_id = ?`, artifactID); err != nil {
		return err
	}

	for _, f := range files {
		if _, err := tx.ExecContext(ctx, `INSERT INTO affected_files (artifact_id, file_path, file_hash) VALUES (?, ?, ?)`,
			artifactID, f.Path, f.Hash); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAffectedFiles returns the affected files for an artifact.
func (s *Store) GetAffectedFiles(ctx context.Context, artifactID string) ([]AffectedFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT file_path, file_hash FROM affected_files WHERE artifact_id = ?`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []AffectedFile
	for rows.Next() {
		var f AffectedFile
		if err := rows.Scan(&f.Path, &f.Hash); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// --- Affected Symbols (tree-sitter powered) ---

// SetAffectedSymbols replaces the symbol snapshots for an artifact.
func (s *Store) SetAffectedSymbols(ctx context.Context, artifactID string, symbols []AffectedSymbol) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM affected_symbols WHERE artifact_id = ?`, artifactID); err != nil {
		return err
	}

	for _, sym := range symbols {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO affected_symbols (artifact_id, file_path, symbol_name, symbol_kind, symbol_line, symbol_end_line, symbol_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			artifactID, sym.FilePath, sym.SymbolName, sym.SymbolKind, sym.Line, sym.EndLine, sym.Hash); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAffectedSymbols returns the symbol snapshots for an artifact.
func (s *Store) GetAffectedSymbols(ctx context.Context, artifactID string) ([]AffectedSymbol, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT file_path, symbol_name, symbol_kind, symbol_line, symbol_end_line, symbol_hash
		 FROM affected_symbols WHERE artifact_id = ? ORDER BY file_path, symbol_line`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []AffectedSymbol
	for rows.Next() {
		var sym AffectedSymbol
		if err := rows.Scan(&sym.FilePath, &sym.SymbolName, &sym.SymbolKind, &sym.Line, &sym.EndLine, &sym.Hash); err != nil {
			return nil, err
		}
		symbols = append(symbols, sym)
	}
	return symbols, rows.Err()
}

// --- Evidence Items ---

const cl0EvidenceSupportsError = "CL0 evidence cannot support — re-evaluate or change verdict"

// AddEvidenceItem adds an evidence item linked to an artifact.
func (s *Store) AddEvidenceItem(ctx context.Context, item *EvidenceItem, artifactRef string) error {
	return s.addEvidenceItemWithExec(ctx, s.db, item, artifactRef)
}

func (s *Store) addEvidenceItemWithExec(ctx context.Context, execer sqlExecer, item *EvidenceItem, artifactRef string) error {
	formalityScale := storedEvidenceFormalityScale(item, item.FormalityLevel)
	formality := formalityScale.Level
	storedVerdict := canonicalStoredEvidenceVerdict(item.Type, item.Verdict)
	err := validateEvidenceCongruenceAtIngest(storedVerdict, item.CongruenceLevel)
	if err != nil {
		return err
	}
	hasClaimScope, err := s.tableHasColumn(ctx, "evidence_items", "claim_scope")
	if err != nil {
		return err
	}
	hasClaimRefs, err := s.tableHasColumn(ctx, "evidence_items", "claim_refs")
	if err != nil {
		return err
	}
	hasProvenance, err := s.tableHasColumn(ctx, "evidence_items", "provenance")
	if err != nil {
		return err
	}
	hasFormalityScaleID, err := s.tableHasColumn(ctx, "evidence_items", "formality_scale_id")
	if err != nil {
		return err
	}
	hasFormalityBridge, err := s.tableHasColumn(ctx, "evidence_items", "formality_bridge")
	if err != nil {
		return err
	}

	scopeJSON := "[]"
	if scope := normalizeClaimScope(item.ClaimScope); len(scope) > 0 {
		data, err := json.Marshal(scope)
		if err != nil {
			return fmt.Errorf("marshal claim_scope: %w", err)
		}
		scopeJSON = string(data)
	}

	claimRefsJSON := "[]"
	if claimRefs := normalizeClaimRefs(item.ClaimRefs); len(claimRefs) > 0 {
		data, err := json.Marshal(claimRefs)
		if err != nil {
			return fmt.Errorf("marshal claim_refs: %w", err)
		}
		claimRefsJSON = string(data)
	}
	formalityBridge := item.FormalityBridge
	if formalityBridge == nil {
		formalityBridge = evidenceFormalityBridge(formalityScale)
	}
	formalityBridgeJSON := ""
	if formalityBridge != nil {
		data, err := json.Marshal(formalityBridge)
		if err != nil {
			return fmt.Errorf("marshal formality_bridge: %w", err)
		}
		formalityBridgeJSON = string(data)
	}

	columns := []string{
		"id",
		"artifact_ref",
		"type",
		"content",
		"verdict",
		"carrier_ref",
		"congruence_level",
		"formality_level",
	}
	args := []any{
		item.ID, artifactRef, item.Type, item.Content, storedVerdict,
		item.CarrierRef, item.CongruenceLevel, formality,
	}
	if hasClaimScope {
		columns = append(columns, "claim_scope")
		args = append(args, scopeJSON)
	}
	if hasClaimRefs {
		columns = append(columns, "claim_refs")
		args = append(args, claimRefsJSON)
	}
	if hasProvenance {
		columns = append(columns, "provenance")
		args = append(args, normalizeEvidenceProvenance(item.Provenance))
	}
	if hasFormalityScaleID {
		columns = append(columns, "formality_scale_id")
		args = append(args, formalityScale.ScaleID)
	}
	if hasFormalityBridge {
		columns = append(columns, "formality_bridge")
		args = append(args, formalityBridgeJSON)
	}

	columns = append(columns, "valid_until", "created_at")
	args = append(args, item.ValidUntil, time.Now().UTC().Format(time.RFC3339))

	placeholders := make([]string, len(columns))
	for index := range placeholders {
		placeholders[index] = "?"
	}

	query := fmt.Sprintf(
		"INSERT INTO evidence_items (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err = execer.ExecContext(ctx, query, args...)
	item.Verdict = storedVerdict
	item.FormalityLevel = formality
	item.FormalityScale = &formalityScale
	item.FormalityBridge = formalityBridge
	return err
}

// GetEvidenceItems returns evidence items for an artifact.
func (s *Store) GetEvidenceItems(ctx context.Context, artifactRef string) ([]EvidenceItem, error) {
	return s.getEvidenceItemsWithQueryer(ctx, s.db, artifactRef)
}

func (s *Store) getEvidenceItemsWithQueryer(ctx context.Context, queryer sqlQueryer, artifactRef string) ([]EvidenceItem, error) {
	hasClaimScope, err := s.tableHasColumn(ctx, "evidence_items", "claim_scope")
	if err != nil {
		return nil, err
	}
	hasClaimRefs, err := s.tableHasColumn(ctx, "evidence_items", "claim_refs")
	if err != nil {
		return nil, err
	}
	hasProvenance, err := s.tableHasColumn(ctx, "evidence_items", "provenance")
	if err != nil {
		return nil, err
	}
	hasFormalityScaleID, err := s.tableHasColumn(ctx, "evidence_items", "formality_scale_id")
	if err != nil {
		return nil, err
	}
	hasFormalityBridge, err := s.tableHasColumn(ctx, "evidence_items", "formality_bridge")
	if err != nil {
		return nil, err
	}

	columns := []string{
		"id",
		"type",
		"content",
		"verdict",
		"carrier_ref",
		"congruence_level",
		"formality_level",
	}
	if hasClaimScope {
		columns = append(columns, "claim_scope")
	}
	if hasClaimRefs {
		columns = append(columns, "claim_refs")
	}
	if hasProvenance {
		columns = append(columns, "provenance")
	}
	if hasFormalityScaleID {
		columns = append(columns, "formality_scale_id")
	}
	if hasFormalityBridge {
		columns = append(columns, "formality_bridge")
	}
	columns = append(columns, "valid_until")

	query := "SELECT " +
		strings.Join(columns, ", ") +
		" FROM evidence_items WHERE artifact_ref = ? ORDER BY created_at DESC"

	rows, err := queryer.QueryContext(ctx, query, artifactRef)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []EvidenceItem
	for rows.Next() {
		var e EvidenceItem
		var verdict, carrierRef, claimScope, claimRefs, provenance sql.NullString
		var formalityScaleID, formalityBridge, validUntil sql.NullString
		dest := []any{
			&e.ID,
			&e.Type,
			&e.Content,
			&verdict,
			&carrierRef,
			&e.CongruenceLevel,
			&e.FormalityLevel,
		}
		if hasClaimScope {
			dest = append(dest, &claimScope)
		}
		if hasClaimRefs {
			dest = append(dest, &claimRefs)
		}
		if hasProvenance {
			dest = append(dest, &provenance)
		}
		if hasFormalityScaleID {
			dest = append(dest, &formalityScaleID)
		}
		if hasFormalityBridge {
			dest = append(dest, &formalityBridge)
		}
		dest = append(dest, &validUntil)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		e.Provenance = normalizeEvidenceProvenance(provenance.String)
		e.Verdict = canonicalStoredEvidenceVerdict(e.Type, verdict.String)
		e.CarrierRef = carrierRef.String
		e.FormalityLevel = normalizeFormalityLevel(e.FormalityLevel)
		e.FormalityScale = readEvidenceFormalityScale(e.FormalityLevel, formalityScaleID.String)
		e.FormalityBridge = readEvidenceFormalityBridge(e.FormalityLevel, formalityScaleID.String, formalityBridge.String)
		if claimScope.String != "" {
			_ = json.Unmarshal([]byte(claimScope.String), &e.ClaimScope)
			e.ClaimScope = normalizeClaimScope(e.ClaimScope)
		}
		if claimRefs.String != "" {
			_ = json.Unmarshal([]byte(claimRefs.String), &e.ClaimRefs)
			e.ClaimRefs = normalizeClaimRefs(e.ClaimRefs)
		}
		e.ValidUntil = validUntil.String
		items = append(items, e)
	}
	return items, rows.Err()
}

func canonicalStoredEvidenceVerdict(evidenceType string, verdict string) string {
	normalizedVerdict := strings.TrimSpace(verdict)
	if !strings.EqualFold(strings.TrimSpace(evidenceType), "measurement") {
		return normalizedVerdict
	}

	switch normalizedVerdict {
	case "accepted":
		return "supports"
	case "partial":
		return "weakens"
	case "failed":
		return "refutes"
	default:
		return normalizedVerdict
	}
}

func validateEvidenceCongruenceAtIngest(storedVerdict string, congruenceLevel int) error {
	if congruenceLevel == 0 && strings.EqualFold(strings.TrimSpace(storedVerdict), "supports") {
		return fmt.Errorf("%s", cl0EvidenceSupportsError)
	}

	return nil
}

func storedEvidenceFormalityScale(item *EvidenceItem, level int) reff.FormalityScale {
	if item.FormalityScale == nil {
		return reff.CurrentFormalityScale(level)
	}

	scale := *item.FormalityScale
	scale.Level = level
	return reff.NormalizeFormalityScale(scale)
}

func readEvidenceFormalityScale(level int, scaleID string) *reff.FormalityScale {
	trimmed := strings.TrimSpace(scaleID)
	if trimmed != "" {
		scale := reff.NormalizeFormalityScale(reff.FormalityScale{
			ScaleID: trimmed,
			Level:   level,
		})
		return &scale
	}

	scale := reff.UnversionedFormalityScale(level)
	if level >= 0 && level <= 3 {
		scale = reff.LegacyFormalityScale(level)
	}

	return &scale
}

func readEvidenceFormalityBridge(level int, scaleID string, rawBridge string) *reff.FormalityBridge {
	trimmedBridge := strings.TrimSpace(rawBridge)
	if trimmedBridge != "" {
		var bridge reff.FormalityBridge
		if err := json.Unmarshal([]byte(trimmedBridge), &bridge); err == nil {
			return &bridge
		}
	}

	scale := readEvidenceFormalityScale(level, scaleID)
	if scale == nil {
		return nil
	}

	return evidenceFormalityBridge(*scale)
}

// SupersedeEvidenceByType marks all evidence items of the given type on an artifact as superseded.
// Used by Measure to supersede previous measurements (FPF F.10:6.1 — newer evidence replaces older).
func (s *Store) SupersedeEvidenceByType(ctx context.Context, artifactRef string, evidenceType string) error {
	return s.supersedeEvidenceByTypeWithExec(ctx, s.db, artifactRef, evidenceType)
}

func (s *Store) supersedeEvidenceByTypeWithExec(ctx context.Context, execer sqlExecer, artifactRef string, evidenceType string) error {
	_, err := execer.ExecContext(ctx,
		`UPDATE evidence_items SET verdict = 'superseded' WHERE artifact_ref = ? AND type = ? AND verdict != 'superseded'`,
		artifactRef, evidenceType)
	return err
}

func (s *Store) supersedeEvidenceByIDWithExec(ctx context.Context, execer sqlExecer, ids []string) error {
	for _, id := range ids {
		_, err := execer.ExecContext(ctx,
			`UPDATE evidence_items SET verdict = 'superseded' WHERE id = ? AND verdict != 'superseded'`,
			id)
		if err != nil {
			return err
		}
	}

	return nil
}

func measurementBindingKeys(claims []DecisionClaim, item EvidenceItem) []string {
	normalizedClaims := normalizeDecisionClaims(claims)
	if len(normalizedClaims) == 0 {
		return nil
	}

	resolvedRefs, err := resolveDecisionEvidenceClaimRefs(
		normalizedClaims,
		item.ClaimRefs,
		item.ClaimScope,
	)
	if err != nil || len(resolvedRefs) == 0 {
		return nil
	}

	claimIndex := make(map[string]DecisionClaim, len(normalizedClaims))
	keys := make([]string, 0, len(resolvedRefs))

	for _, claim := range normalizedClaims {
		claimIndex[claim.ID] = claim
	}

	for _, ref := range normalizeClaimRefs(resolvedRefs) {
		claim, ok := claimIndex[ref]
		if !ok {
			continue
		}

		key := strings.TrimSpace(ref) + "\x00" + strings.TrimSpace(claim.Observable)
		keys = append(keys, key)
	}

	return keys
}

func measurementKeysOverlap(left []string, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return len(left) == 0 && len(right) == 0
	}

	index := make(map[string]struct{}, len(left))

	for _, value := range left {
		index[value] = struct{}{}
	}

	for _, value := range right {
		if _, ok := index[value]; ok {
			return true
		}
	}

	return false
}

func (s *Store) measurementEvidenceIDsToSupersede(
	ctx context.Context,
	queryer sqlQueryer,
	decision *Artifact,
	items []EvidenceItem,
) ([]string, error) {
	existingItems, err := s.getEvidenceItemsWithQueryer(ctx, queryer, decision.Meta.ID)
	if err != nil {
		return nil, err
	}

	decisionClaims := decision.UnmarshalDecisionFields().Claims
	incomingKeys := make([][]string, 0, len(items))
	ids := make([]string, 0)

	for _, item := range items {
		incomingKeys = append(incomingKeys, measurementBindingKeys(decisionClaims, item))
	}

	for _, existing := range existingItems {
		if existing.Type != "measurement" || existing.Verdict == "superseded" {
			continue
		}

		existingKeys := measurementBindingKeys(decisionClaims, existing)

		for _, keys := range incomingKeys {
			if !measurementKeysOverlap(existingKeys, keys) {
				continue
			}

			ids = append(ids, existing.ID)
			break
		}
	}

	return ids, nil
}

// CommitMeasurement atomically updates a decision and supersedes only overlapping active measurement evidence.
func (s *Store) CommitMeasurement(ctx context.Context, decision *Artifact, items []EvidenceItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.updateArtifactWithExec(ctx, tx, decision); err != nil {
		return err
	}

	supersededIDs, err := s.measurementEvidenceIDsToSupersede(ctx, tx, decision, items)
	if err != nil {
		return err
	}
	if err := s.supersedeEvidenceByIDWithExec(ctx, tx, supersededIDs); err != nil {
		return err
	}

	for _, item := range items {
		item := item
		if err := s.addEvidenceItemWithExec(ctx, tx, &item, decision.Meta.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LastRefreshScan returns the timestamp of the last haft_refresh:scan call from audit_log.
// Returns zero time if never scanned.
func (s *Store) LastRefreshScan(ctx context.Context) time.Time {
	var ts string
	err := s.db.QueryRowContext(ctx,
		`SELECT timestamp FROM audit_log WHERE operation = 'haft_refresh:scan' ORDER BY timestamp DESC LIMIT 1`,
	).Scan(&ts)
	if err != nil {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, ts)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", ts) // SQLite CURRENT_TIMESTAMP format
	}
	return t
}

// EpistemicDebtBudget returns the configured ED budget, or the default when
// the shared state table or column is unavailable.
func (s *Store) EpistemicDebtBudget(ctx context.Context) (float64, error) {
	hasColumn, err := s.tableHasColumn(ctx, "fpf_state", "epistemic_debt_budget")
	if err != nil {
		return DefaultEpistemicDebtBudget, err
	}
	if !hasColumn {
		return DefaultEpistemicDebtBudget, nil
	}

	var budget sql.NullFloat64
	err = s.db.QueryRowContext(ctx, `
		SELECT epistemic_debt_budget
		FROM fpf_state
		ORDER BY updated_at DESC
		LIMIT 1`,
	).Scan(&budget)
	if err == sql.ErrNoRows {
		return DefaultEpistemicDebtBudget, nil
	}
	if err != nil {
		return DefaultEpistemicDebtBudget, fmt.Errorf("query epistemic debt budget: %w", err)
	}
	if !budget.Valid {
		return DefaultEpistemicDebtBudget, nil
	}
	if budget.Float64 < 0 {
		return 0, nil
	}

	return budget.Float64, nil
}

// --- helpers ---

func (s *Store) tableHasColumn(ctx context.Context, tableName, columnName string) (bool, error) {
	cacheKey := tableName + "." + columnName
	if result, ok := s.colCache[cacheKey]; ok {
		return result, nil
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, fmt.Errorf("inspect table %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			kind       string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)

		err := rows.Scan(&cid, &name, &kind, &notNull, &defaultVal, &primaryKey)
		if err != nil {
			return false, fmt.Errorf("scan table info %s: %w", tableName, err)
		}
		if name == columnName {
			s.colCache[cacheKey] = true
			return true, nil
		}
	}

	s.colCache[cacheKey] = false
	return false, nil
}

func scanArtifacts(rows *sql.Rows) ([]*Artifact, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []*Artifact
	for rows.Next() {
		var a Artifact
		var kind, status, mode, validUntil, ctx, createdAt, updatedAt string
		var structuredData sql.NullString
		destinations := []any{
			&a.Meta.ID,
			&kind,
			&a.Meta.Version,
			&status,
			&ctx,
			&mode,
			&a.Meta.Title,
			&a.Body,
			&validUntil,
			&createdAt,
			&updatedAt,
		}
		if len(columns) == len(destinations)+1 && columns[len(columns)-1] == "structured_data" {
			destinations = append(destinations, &structuredData)
		}
		if len(columns) != len(destinations) {
			return nil, fmt.Errorf("scan artifacts: unexpected column count %d", len(columns))
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		a.Meta.Kind, _ = ParseKind(kind)
		a.Meta.Status, _ = ParseStatus(status)
		a.Meta.Mode, _ = ParseMode(mode)
		a.Meta.Context = ctx
		a.Meta.ValidUntil = validUntil
		a.Meta.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		a.Meta.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if structuredData.Valid {
			a.StructuredData = structuredData.String
		}
		result = append(result, &a)
	}
	return result, rows.Err()
}

func filterStaleArtifacts(artifacts []*Artifact, now time.Time) []*Artifact {
	filtered := make([]*Artifact, 0, len(artifacts))

	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		if artifact.Meta.Status == StatusRefreshDue {
			filtered = append(filtered, artifact)
			continue
		}
		if !isExpiredValidUntil(artifact.Meta.ValidUntil, now) {
			continue
		}
		filtered = append(filtered, artifact)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := staleSortKey(filtered[i].Meta.ValidUntil)
		right := staleSortKey(filtered[j].Meta.ValidUntil)
		if !left.Equal(right) {
			return left.Before(right)
		}
		if filtered[i].Meta.Kind != filtered[j].Meta.Kind {
			return filtered[i].Meta.Kind < filtered[j].Meta.Kind
		}
		return filtered[i].Meta.ID < filtered[j].Meta.ID
	})

	return filtered
}

func isExpiredValidUntil(validUntil string, now time.Time) bool {
	expiry, ok := reff.ParseValidUntil(validUntil)
	if !ok {
		return false
	}
	return expiry.Before(now)
}

func staleSortKey(validUntil string) time.Time {
	expiry, ok := reff.ParseValidUntil(validUntil)
	if !ok {
		return time.Time{}
	}
	return expiry
}
