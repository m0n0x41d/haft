package artifact

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Store handles artifact persistence in SQLite.
type Store struct {
	db *sql.DB
}

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
		INSERT INTO artifacts (id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at, search_keywords)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Meta.ID, string(a.Meta.Kind), a.Meta.Version, string(a.Meta.Status),
		a.Meta.Context, string(a.Meta.Mode), a.Meta.Title, a.Body,
		a.Meta.ValidUntil,
		a.Meta.CreatedAt.Format(time.RFC3339),
		a.Meta.UpdatedAt.Format(time.RFC3339),
		a.SearchKeywords,
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
	var searchKeywords sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at, COALESCE(search_keywords, '')
		FROM artifacts WHERE id = ?`, id,
	).Scan(
		&a.Meta.ID, &kind, &a.Meta.Version, &status, &context_, &mode,
		&a.Meta.Title, &a.Body, &validUntil, &createdAt, &updatedAt, &searchKeywords,
	)
	if err != nil {
		return nil, fmt.Errorf("get artifact %s: %w", id, err)
	}
	a.Meta.Kind = Kind(kind)
	a.Meta.Status = Status(status)
	a.Meta.Mode = Mode(mode)
	a.Meta.Context = context_
	a.Meta.ValidUntil = validUntil
	a.Meta.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.Meta.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	a.SearchKeywords = searchKeywords.String

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
		UPDATE artifacts SET kind=?, version=?, status=?, context=?, mode=?, title=?, content=?, valid_until=?, updated_at=?, search_keywords=?
		WHERE id=?`,
		string(a.Meta.Kind), a.Meta.Version, string(a.Meta.Status),
		a.Meta.Context, string(a.Meta.Mode), a.Meta.Title, a.Body,
		a.Meta.ValidUntil, a.Meta.UpdatedAt.Format(time.RFC3339),
		a.SearchKeywords,
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
func (s *Store) ListByKind(ctx context.Context, kind Kind, limit int) ([]*Artifact, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
		FROM artifacts WHERE kind = ? ORDER BY created_at DESC LIMIT ?`,
		string(kind), limit,
	)
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
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
		FROM artifacts WHERE status NOT IN ('superseded', 'deprecated') ORDER BY updated_at DESC LIMIT ?`,
		limit,
	)
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
	ftsQuery := strings.Join(ftsTerms, " OR ")

	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.kind, a.version, a.status, a.context, a.mode, a.title, a.content, a.valid_until, a.created_at, a.updated_at
		FROM artifacts a
		JOIN artifacts_fts f ON a.id = f.id
		WHERE artifacts_fts MATCH ?
		ORDER BY rank
		LIMIT ?`,
		ftsQuery, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	return scanArtifacts(rows)
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
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
		FROM artifacts
		WHERE kind = ? AND (
			status = 'refresh_due'
			OR (valid_until != '' AND valid_until < ?)
		)
		ORDER BY valid_until ASC`,
		string(KindDecisionRecord), now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
}

// FindStaleArtifacts returns any artifacts (not just decisions) past their valid_until.
// This catches stale ProblemCards, expired characterizations, and old portfolios.
func (s *Store) FindStaleArtifacts(ctx context.Context) ([]*Artifact, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, version, status, context, mode, title, content, valid_until, created_at, updated_at
		FROM artifacts
		WHERE status NOT IN ('superseded', 'deprecated')
		AND valid_until != '' AND valid_until < ?
		ORDER BY kind, valid_until ASC`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArtifacts(rows)
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

// --- Evidence Items ---

// AddEvidenceItem adds an evidence item linked to an artifact.
func (s *Store) AddEvidenceItem(ctx context.Context, item *EvidenceItem, artifactRef string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO evidence_items (id, artifact_ref, type, content, verdict, carrier_ref, congruence_level, formality_level, valid_until, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, artifactRef, item.Type, item.Content, item.Verdict,
		item.CarrierRef, item.CongruenceLevel, item.FormalityLevel,
		item.ValidUntil, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetEvidenceItems returns evidence items for an artifact.
func (s *Store) GetEvidenceItems(ctx context.Context, artifactRef string) ([]EvidenceItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, content, verdict, carrier_ref, congruence_level, formality_level, valid_until
		FROM evidence_items WHERE artifact_ref = ? ORDER BY created_at DESC`, artifactRef)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []EvidenceItem
	for rows.Next() {
		var e EvidenceItem
		var verdict, carrierRef, validUntil sql.NullString
		if err := rows.Scan(&e.ID, &e.Type, &e.Content, &verdict, &carrierRef,
			&e.CongruenceLevel, &e.FormalityLevel, &validUntil); err != nil {
			return nil, err
		}
		e.Verdict = verdict.String
		e.CarrierRef = carrierRef.String
		e.ValidUntil = validUntil.String
		items = append(items, e)
	}
	return items, rows.Err()
}

// LastRefreshScan returns the timestamp of the last quint_refresh:scan call from audit_log.
// Returns zero time if never scanned.
func (s *Store) LastRefreshScan(ctx context.Context) time.Time {
	var ts string
	err := s.db.QueryRowContext(ctx,
		`SELECT timestamp FROM audit_log WHERE operation = 'quint_refresh:scan' ORDER BY timestamp DESC LIMIT 1`,
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

// --- helpers ---

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
		a.Meta.Kind = Kind(kind)
		a.Meta.Status = Status(status)
		a.Meta.Mode = Mode(mode)
		a.Meta.Context = ctx
		a.Meta.ValidUntil = validUntil
		a.Meta.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		a.Meta.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		result = append(result, &a)
	}
	return result, rows.Err()
}
