package artifact

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

// Store handles artifact persistence in SQLite.
// Implements ArtifactStore interface.
type Store struct {
	db *sql.DB
}

// Compile-time check: Store must implement ArtifactStore.
var _ ArtifactStore = (*Store)(nil)

// NewStore creates a new artifact store using an existing DB connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
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

	_, err := s.db.ExecContext(ctx, `
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
		if err := s.AddLink(ctx, a.Meta.ID, link.Ref, link.Type); err != nil {
			return fmt.Errorf("insert link %s→%s: %w", a.Meta.ID, link.Ref, err)
		}
	}

	return nil
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
	a.Meta.UpdatedAt = time.Now().UTC()
	a.Meta.Version++

	result, err := s.db.ExecContext(ctx, `
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

// Search performs FTS5 full-text search across artifacts.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]*Artifact, error) {
	if limit <= 0 {
		limit = 20
	}

	terms := strings.Fields(query)

	var ftsTerms []string
	for _, t := range terms {
		// Strip all FTS5 special/operator characters that break queries
		t = strings.NewReplacer(
			`"`, ``, `*`, ``, `(`, ``, `)`, ``,
			`{`, ``, `}`, ``, `^`, ``, `+`, ``,
			`-`, ``, `:`, ``, `~`, ``, `'`, ``,
		).Replace(t)
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		// Quoting treats everything as literal (no FTS5 operator interpretation)
		ftsTerms = append(ftsTerms, fmt.Sprintf(`"%s"*`, t))
	}
	if len(ftsTerms) == 0 {
		return nil, nil
	}

	searchQuery := `
		SELECT a.id, a.kind, a.version, a.status, a.context, a.mode, a.title, a.content, a.valid_until, a.created_at, a.updated_at
		FROM artifacts a
		JOIN artifacts_fts f ON a.id = f.id
		WHERE artifacts_fts MATCH ?
		ORDER BY bm25(artifacts_fts, 0.0, 10.0, 1.0, 5.0, 3.0)
		LIMIT ?`

	// AND-default: require all terms present (implicit AND = space-join in FTS5)
	ftsQuery := strings.Join(ftsTerms, " ")
	rows, err := s.db.QueryContext(ctx, searchQuery, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	results, err := scanArtifacts(rows)
	_ = rows.Close()
	if err != nil {
		return nil, err
	}

	// Fallback to OR if AND returned nothing
	if len(results) == 0 && len(ftsTerms) > 1 {
		ftsQuery = strings.Join(ftsTerms, " OR ")
		rows, err = s.db.QueryContext(ctx, searchQuery, ftsQuery, limit)
		if err != nil {
			return nil, fmt.Errorf("search fallback: %w", err)
		}
		defer rows.Close()
		return scanArtifacts(rows)
	}

	return results, nil
}

// SearchByAffectedFile finds artifacts linked to a specific file path.
func (s *Store) SearchByAffectedFile(ctx context.Context, filePath string) ([]*Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.kind, a.version, a.status, a.context, a.mode, a.title, a.content, a.valid_until, a.created_at, a.updated_at
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

// AddEvidenceItem adds an evidence item linked to an artifact.
func (s *Store) AddEvidenceItem(ctx context.Context, item *EvidenceItem, artifactRef string) error {
	formality := normalizeFormalityLevel(item.FormalityLevel)
	scopeJSON := "[]"
	if scope := normalizeClaimScope(item.ClaimScope); len(scope) > 0 {
		data, err := json.Marshal(scope)
		if err != nil {
			return fmt.Errorf("marshal claim_scope: %w", err)
		}
		scopeJSON = string(data)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO evidence_items (id, artifact_ref, type, content, verdict, carrier_ref, congruence_level, formality_level, claim_scope, valid_until, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, artifactRef, item.Type, item.Content, item.Verdict,
		item.CarrierRef, item.CongruenceLevel, formality, scopeJSON,
		item.ValidUntil, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetEvidenceItems returns evidence items for an artifact.
func (s *Store) GetEvidenceItems(ctx context.Context, artifactRef string) ([]EvidenceItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, content, verdict, carrier_ref, congruence_level, formality_level, claim_scope, valid_until
		FROM evidence_items WHERE artifact_ref = ? ORDER BY created_at DESC`, artifactRef)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []EvidenceItem
	for rows.Next() {
		var e EvidenceItem
		var verdict, carrierRef, claimScope, validUntil sql.NullString
		if err := rows.Scan(&e.ID, &e.Type, &e.Content, &verdict, &carrierRef,
			&e.CongruenceLevel, &e.FormalityLevel, &claimScope, &validUntil); err != nil {
			return nil, err
		}
		e.Verdict = verdict.String
		e.CarrierRef = carrierRef.String
		e.FormalityLevel = normalizeFormalityLevel(e.FormalityLevel)
		if claimScope.String != "" {
			_ = json.Unmarshal([]byte(claimScope.String), &e.ClaimScope)
			e.ClaimScope = normalizeClaimScope(e.ClaimScope)
		}
		e.ValidUntil = validUntil.String
		items = append(items, e)
	}
	return items, rows.Err()
}

// SupersedeEvidenceByType marks all evidence items of the given type on an artifact as superseded.
// Used by Measure to supersede previous measurements (FPF F.10:6.1 — newer evidence replaces older).
func (s *Store) SupersedeEvidenceByType(ctx context.Context, artifactRef string, evidenceType string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE evidence_items SET verdict = 'superseded' WHERE artifact_ref = ? AND type = ? AND verdict != 'superseded'`,
		artifactRef, evidenceType)
	return err
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
			return true, nil
		}
	}

	return false, nil
}

func scanArtifacts(rows *sql.Rows) ([]*Artifact, error) {
	var result []*Artifact
	for rows.Next() {
		var a Artifact
		var kind, status, mode, validUntil, ctx, createdAt, updatedAt string
		if err := rows.Scan(
			&a.Meta.ID, &kind, &a.Meta.Version, &status, &ctx, &mode,
			&a.Meta.Title, &a.Body, &validUntil, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		a.Meta.Kind, _ = ParseKind(kind)
		a.Meta.Status, _ = ParseStatus(status)
		a.Meta.Mode, _ = ParseMode(mode)
		a.Meta.Context = ctx
		a.Meta.ValidUntil = validUntil
		a.Meta.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		a.Meta.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
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
