package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS holons (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	kind TEXT,
	layer TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	context_id TEXT NOT NULL,
	scope TEXT,
	parent_id TEXT REFERENCES holons(id),
	cached_r_score REAL DEFAULT 0.0 CHECK(cached_r_score BETWEEN 0.0 AND 1.0),
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS evidence (
	id TEXT PRIMARY KEY,
	holon_id TEXT NOT NULL,
	type TEXT NOT NULL,
	content TEXT NOT NULL,
	verdict TEXT NOT NULL,
	assurance_level TEXT,
	carrier_ref TEXT,
	valid_until DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS relations (
	source_id TEXT NOT NULL,
	target_id TEXT NOT NULL,
	relation_type TEXT NOT NULL,
	congruence_level INTEGER DEFAULT 3 CHECK(congruence_level BETWEEN 0 AND 3),
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (source_id, target_id, relation_type)
);
CREATE TABLE IF NOT EXISTS characteristics (
	id TEXT PRIMARY KEY,
	holon_id TEXT NOT NULL,
	name TEXT NOT NULL,
	scale TEXT NOT NULL,
	value TEXT NOT NULL,
	unit TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(holon_id) REFERENCES holons(id)
);
CREATE TABLE IF NOT EXISTS work_records (
	id TEXT PRIMARY KEY,
	method_ref TEXT NOT NULL,
	performer_ref TEXT NOT NULL,
	started_at DATETIME NOT NULL,
	ended_at DATETIME,
	resource_ledger TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS audit_log (
	id TEXT PRIMARY KEY,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	tool_name TEXT NOT NULL,
	operation TEXT NOT NULL,
	actor TEXT NOT NULL,
	target_id TEXT,
	input_hash TEXT,
	result TEXT NOT NULL,
	details TEXT,
	context_id TEXT NOT NULL DEFAULT 'default'
);
CREATE TABLE IF NOT EXISTS waivers (
	id TEXT PRIMARY KEY,
	evidence_id TEXT NOT NULL,
	waived_by TEXT NOT NULL,
	waived_until DATETIME NOT NULL,
	rationale TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(evidence_id) REFERENCES evidence(id)
);
CREATE INDEX IF NOT EXISTS idx_relations_target ON relations(target_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_relations_source ON relations(source_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_waivers_evidence ON waivers(evidence_id);

-- FTS5 virtual tables for full-text search
CREATE VIRTUAL TABLE IF NOT EXISTS holons_fts USING fts5(
	id,
	title,
	content,
	content='holons',
	content_rowid='rowid'
);

CREATE VIRTUAL TABLE IF NOT EXISTS evidence_fts USING fts5(
	id,
	content,
	content='evidence',
	content_rowid='rowid'
);

-- Triggers to keep FTS in sync with holons
CREATE TRIGGER IF NOT EXISTS holons_ai AFTER INSERT ON holons BEGIN
	INSERT INTO holons_fts(rowid, id, title, content)
	VALUES (new.rowid, new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS holons_ad AFTER DELETE ON holons BEGIN
	INSERT INTO holons_fts(holons_fts, rowid, id, title, content)
	VALUES('delete', old.rowid, old.id, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS holons_au AFTER UPDATE ON holons BEGIN
	INSERT INTO holons_fts(holons_fts, rowid, id, title, content)
	VALUES('delete', old.rowid, old.id, old.title, old.content);
	INSERT INTO holons_fts(rowid, id, title, content)
	VALUES (new.rowid, new.id, new.title, new.content);
END;

-- Triggers to keep FTS in sync with evidence
CREATE TRIGGER IF NOT EXISTS evidence_ai AFTER INSERT ON evidence BEGIN
	INSERT INTO evidence_fts(rowid, id, content)
	VALUES (new.rowid, new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS evidence_ad AFTER DELETE ON evidence BEGIN
	INSERT INTO evidence_fts(evidence_fts, rowid, id, content)
	VALUES('delete', old.rowid, old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS evidence_au AFTER UPDATE ON evidence BEGIN
	INSERT INTO evidence_fts(evidence_fts, rowid, id, content)
	VALUES('delete', old.rowid, old.id, old.content);
	INSERT INTO evidence_fts(rowid, id, content)
	VALUES (new.rowid, new.id, new.content);
END;
`

type Store struct {
	conn *sql.DB
	q    *Queries
}

func NewStore(dbPath string) (*Store, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to init schema: %v", err)
	}

	if err := RunMigrations(conn); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return &Store{
		conn: conn,
		q:    New(),
	}, nil
}

func (s *Store) GetRawDB() *sql.DB {
	return s.conn
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) CreateHolon(ctx context.Context, id, typ, kind, layer, title, content, contextID, scope, parentID string) error {
	now := sql.NullTime{Time: time.Now(), Valid: true}
	return s.q.CreateHolon(ctx, s.conn, CreateHolonParams{
		ID:        id,
		Type:      typ,
		Kind:      toNullString(kind),
		Layer:     layer,
		Title:     title,
		Content:   content,
		ContextID: contextID,
		Scope:     toNullString(scope),
		ParentID:  toNullString(parentID),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *Store) GetHolon(ctx context.Context, id string) (Holon, error) {
	return s.q.GetHolon(ctx, s.conn, id)
}

func (s *Store) GetHolonTitle(ctx context.Context, id string) (string, error) {
	return s.q.GetHolonTitle(ctx, s.conn, id)
}

func (s *Store) ListAllHolonIDs(ctx context.Context) ([]string, error) {
	return s.q.ListAllHolonIDs(ctx, s.conn)
}

func (s *Store) UpdateHolonLayer(ctx context.Context, id, layer string) error {
	return s.q.UpdateHolonLayer(ctx, s.conn, UpdateHolonLayerParams{
		ID:        id,
		Layer:     layer,
		UpdatedAt: sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) RecordWork(ctx context.Context, id, methodRef, performerRef string, startedAt, endedAt time.Time, ledger string) error {
	return s.q.RecordWork(ctx, s.conn, RecordWorkParams{
		ID:             id,
		MethodRef:      methodRef,
		PerformerRef:   performerRef,
		StartedAt:      startedAt,
		EndedAt:        sql.NullTime{Time: endedAt, Valid: true},
		ResourceLedger: toNullString(ledger),
		CreatedAt:      sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) AddEvidence(ctx context.Context, id, holonID, typ, content, verdict, assuranceLevel, carrierRef, validUntil string) error {
	var vUntil sql.NullTime
	if validUntil != "" {
		t, err := time.Parse(time.RFC3339, validUntil)
		if err != nil {
			t, err = time.Parse("2006-01-02", validUntil)
		}
		if err == nil {
			vUntil = sql.NullTime{Time: t, Valid: true}
		}
	}

	return s.q.AddEvidence(ctx, s.conn, AddEvidenceParams{
		ID:             id,
		HolonID:        holonID,
		Type:           typ,
		Content:        content,
		Verdict:        verdict,
		AssuranceLevel: toNullString(assuranceLevel),
		CarrierRef:     toNullString(carrierRef),
		ValidUntil:     vUntil,
		CreatedAt:      sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) GetEvidence(ctx context.Context, holonID string) ([]Evidence, error) {
	return s.q.GetEvidenceByHolon(ctx, s.conn, holonID)
}

func (s *Store) GetEvidenceWithCarrier(ctx context.Context) ([]Evidence, error) {
	return s.q.GetEvidenceWithCarrier(ctx, s.conn)
}

func (s *Store) Link(ctx context.Context, source, target, relType string) error {
	return s.q.AddRelation(ctx, s.conn, AddRelationParams{
		SourceID:     source,
		TargetID:     target,
		RelationType: relType,
		CreatedAt:    sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) CreateRelation(ctx context.Context, sourceID, relationType, targetID string, cl int) error {
	return s.q.CreateRelation(ctx, s.conn, CreateRelationParams{
		SourceID:        sourceID,
		RelationType:    relationType,
		TargetID:        targetID,
		CongruenceLevel: sql.NullInt64{Int64: int64(cl), Valid: true},
	})
}

func (s *Store) GetComponentsOf(ctx context.Context, targetID string) ([]GetComponentsOfRow, error) {
	return s.q.GetComponentsOf(ctx, s.conn, targetID)
}

func (s *Store) GetCollectionMembers(ctx context.Context, targetID string) ([]GetCollectionMembersRow, error) {
	return s.q.GetCollectionMembers(ctx, s.conn, targetID)
}

func (s *Store) GetDependencies(ctx context.Context, sourceID string) ([]GetDependenciesRow, error) {
	return s.q.GetDependencies(ctx, s.conn, sourceID)
}

func (s *Store) GetHolonsByParent(ctx context.Context, parentID string) ([]Holon, error) {
	return s.q.GetHolonsByParent(ctx, s.conn, toNullString(parentID))
}

func (s *Store) GetHolonLineage(ctx context.Context, id string) ([]GetHolonLineageRow, error) {
	return s.q.GetHolonLineage(ctx, s.conn, id)
}

func (s *Store) CountHolonsByLayer(ctx context.Context, contextID string) ([]CountHolonsByLayerRow, error) {
	return s.q.CountHolonsByLayer(ctx, s.conn, contextID)
}

// CountActiveHolonsByLayer returns counts by layer, excluding holons in resolved decisions.
func (s *Store) CountActiveHolonsByLayer(ctx context.Context) ([]CountActiveHolonsByLayerRow, error) {
	return s.q.CountActiveHolonsByLayer(ctx, s.conn)
}

// CountArchivedHolonsByLayer returns counts by layer for holons in resolved decisions.
func (s *Store) CountArchivedHolonsByLayer(ctx context.Context) ([]CountArchivedHolonsByLayerRow, error) {
	return s.q.CountArchivedHolonsByLayer(ctx, s.conn)
}

// GetActiveRecentHolons returns recent holons not belonging to resolved decisions.
// Uses active_holons view (migration v6) as single source of truth.
func (s *Store) GetActiveRecentHolons(ctx context.Context, limit int) ([]Holon, error) {
	if limit <= 0 {
		limit = 10
	}
	activeHolons, err := s.q.GetActiveRecentHolons(ctx, s.conn, int64(limit))
	if err != nil {
		return nil, err
	}
	// Convert ActiveHolon (from view) to Holon (identical structure)
	holons := make([]Holon, len(activeHolons))
	for i, ah := range activeHolons {
		holons[i] = Holon{
			ID:           ah.ID,
			Type:         ah.Type,
			Kind:         ah.Kind,
			Layer:        ah.Layer,
			Title:        ah.Title,
			Content:      ah.Content,
			ContextID:    ah.ContextID,
			Scope:        ah.Scope,
			ParentID:     ah.ParentID,
			CachedRScore: ah.CachedRScore,
			CreatedAt:    ah.CreatedAt,
			UpdatedAt:    ah.UpdatedAt,
		}
	}
	return holons, nil
}

func (s *Store) GetLatestHolonByContext(ctx context.Context, contextID string) (Holon, error) {
	return s.q.GetLatestHolonByContext(ctx, s.conn, contextID)
}

func (s *Store) InsertAuditLog(ctx context.Context, id, toolName, operation, actor, targetID, inputHash, result, details, contextID string) error {
	return s.q.InsertAuditLog(ctx, s.conn, InsertAuditLogParams{
		ID:        id,
		ToolName:  toolName,
		Operation: operation,
		Actor:     actor,
		TargetID:  toNullString(targetID),
		InputHash: toNullString(inputHash),
		Result:    result,
		Details:   toNullString(details),
		ContextID: contextID,
	})
}

func (s *Store) GetAuditLogByContext(ctx context.Context, contextID string) ([]AuditLog, error) {
	return s.q.GetAuditLogByContext(ctx, s.conn, contextID)
}

func (s *Store) GetAuditLogByTarget(ctx context.Context, targetID string) ([]AuditLog, error) {
	return s.q.GetAuditLogByTarget(ctx, s.conn, toNullString(targetID))
}

func (s *Store) GetRecentAuditLog(ctx context.Context, limit int64) ([]AuditLog, error) {
	return s.q.GetRecentAuditLog(ctx, s.conn, limit)
}

func (s *Store) CreateWaiver(ctx context.Context, id, evidenceID, waivedBy string, waivedUntil time.Time, rationale string) error {
	return s.q.CreateWaiver(ctx, s.conn, CreateWaiverParams{
		ID:          id,
		EvidenceID:  evidenceID,
		WaivedBy:    waivedBy,
		WaivedUntil: waivedUntil,
		Rationale:   rationale,
		CreatedAt:   sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) GetActiveWaiverForEvidence(ctx context.Context, evidenceID string) (Waiver, error) {
	return s.q.GetActiveWaiverForEvidence(ctx, s.conn, evidenceID)
}

func (s *Store) GetAllActiveWaivers(ctx context.Context) ([]Waiver, error) {
	return s.q.GetAllActiveWaivers(ctx, s.conn)
}

func (s *Store) GetEvidenceByID(ctx context.Context, id string) (Evidence, error) {
	return s.q.GetEvidenceByID(ctx, s.conn, id)
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// SearchResult represents a single search hit.
type SearchResult struct {
	ID        string
	Type      string // "holon" or "evidence"
	Title     string
	Snippet   string
	Layer     string
	Scope     string // affected_scope for DRRs (JSON array of file patterns)
	RScore    float64
	UpdatedAt time.Time
}

// sanitizeFTS5Query escapes special FTS5 characters to prevent query parse errors.
// FTS5 treats - * " ( ) etc. as operators. We wrap in quotes for phrase matching.
func sanitizeFTS5Query(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return query
	}
	escaped := strings.ReplaceAll(query, `"`, `""`)
	return `"` + escaped + `"`
}

// buildFTS5ORQuery splits text into words and builds an OR query for FTS5.
// Returns words joined with OR, each word quoted for safety.
// Filters short words (<3 chars) and limits to 10 terms.
func buildFTS5ORQuery(text string) string {
	words := strings.Fields(strings.ToLower(text))
	var terms []string
	seen := make(map[string]bool)

	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}") // Remove punctuation
		if len(clean) < 3 || seen[clean] {
			continue
		}
		seen[clean] = true
		escaped := strings.ReplaceAll(clean, `"`, `""`)
		terms = append(terms, `"`+escaped+`"`)
		if len(terms) >= 10 {
			break
		}
	}

	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " OR ")
}

// SearchOR performs full-text search using OR of individual words.
// Better for semantic matching where any word match is relevant.
func (s *Store) SearchOR(ctx context.Context, text, scope, layerFilter, statusFilter string, limit int) ([]SearchResult, error) {
	orQuery := buildFTS5ORQuery(text)
	if orQuery == "" {
		return nil, nil
	}
	return s.searchHolonsRaw(ctx, orQuery, layerFilter, statusFilter, limit)
}

// Search performs full-text search across holons and evidence.
// scope: "holons", "evidence", "all"
// layerFilter: "L0", "L1", "L2", "" (all layers)
func (s *Store) Search(ctx context.Context, query, scope, layerFilter, statusFilter string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	safeQuery := sanitizeFTS5Query(query)
	var results []SearchResult

	// Search holons
	if scope == "holons" || scope == "all" || scope == "" {
		holonResults, err := s.searchHolons(ctx, safeQuery, layerFilter, statusFilter, limit)
		if err != nil {
			return nil, fmt.Errorf("holon search failed: %w", err)
		}
		results = append(results, holonResults...)
	}

	// Search evidence
	if scope == "evidence" || scope == "all" || scope == "" {
		evidenceResults, err := s.searchEvidence(ctx, safeQuery, limit)
		if err != nil {
			return nil, fmt.Errorf("evidence search failed: %w", err)
		}
		results = append(results, evidenceResults...)
	}

	// Limit total results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *Store) searchHolons(ctx context.Context, query, layerFilter, statusFilter string, limit int) ([]SearchResult, error) {
	var sqlQuery string
	var args []interface{}

	if statusFilter != "" {
		if statusFilter == "open" {
			sqlQuery = `
				SELECT h.id, h.title, h.layer, h.scope, h.cached_r_score, h.updated_at,
				       snippet(holons_fts, 2, '**', '**', '...', 32) as snippet
				FROM holons_fts
				JOIN holons h ON holons_fts.id = h.id
				WHERE holons_fts MATCH ?
				  AND (h.type = 'DRR' OR h.layer = 'DRR')
				  AND NOT EXISTS (
				      SELECT 1 FROM evidence e
				      WHERE e.holon_id = h.id
				        AND e.type IN ('implementation', 'abandonment', 'supersession')
				  )
				ORDER BY rank
				LIMIT ?
			`
			args = []interface{}{query, limit}
		} else {
			evidenceType := map[string]string{
				"implemented": "implementation",
				"abandoned":   "abandonment",
				"superseded":  "supersession",
			}[statusFilter]
			if evidenceType == "" {
				evidenceType = statusFilter
			}
			sqlQuery = `
				SELECT h.id, h.title, h.layer, h.scope, h.cached_r_score, h.updated_at,
				       snippet(holons_fts, 2, '**', '**', '...', 32) as snippet
				FROM holons_fts
				JOIN holons h ON holons_fts.id = h.id
				WHERE holons_fts MATCH ?
				  AND (h.type = 'DRR' OR h.layer = 'DRR')
				  AND EXISTS (
				      SELECT 1 FROM evidence e
				      WHERE e.holon_id = h.id
				        AND e.type = ?
				  )
				ORDER BY rank
				LIMIT ?
			`
			args = []interface{}{query, evidenceType, limit}
		}
	} else if layerFilter != "" {
		sqlQuery = `
			SELECT h.id, h.title, h.layer, h.scope, h.cached_r_score, h.updated_at,
			       snippet(holons_fts, 2, '**', '**', '...', 32) as snippet
			FROM holons_fts
			JOIN holons h ON holons_fts.id = h.id
			WHERE holons_fts MATCH ?
			  AND h.layer = ?
			ORDER BY rank
			LIMIT ?
		`
		args = []interface{}{query, layerFilter, limit}
	} else {
		sqlQuery = `
			SELECT h.id, h.title, h.layer, h.scope, h.cached_r_score, h.updated_at,
			       snippet(holons_fts, 2, '**', '**', '...', 32) as snippet
			FROM holons_fts
			JOIN holons h ON holons_fts.id = h.id
			WHERE holons_fts MATCH ?
			ORDER BY rank
			LIMIT ?
		`
		args = []interface{}{query, limit}
	}

	rows, err := s.conn.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var updatedAt sql.NullTime
		var rScore sql.NullFloat64
		var scope sql.NullString
		if err := rows.Scan(&r.ID, &r.Title, &r.Layer, &scope, &rScore, &updatedAt, &r.Snippet); err != nil {
			continue
		}
		if scope.Valid {
			r.Scope = scope.String
		}
		r.Type = "holon"
		if rScore.Valid {
			r.RScore = rScore.Float64
		}
		if updatedAt.Valid {
			r.UpdatedAt = updatedAt.Time
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// searchHolonsRaw executes a raw FTS5 query without sanitization.
// Used for pre-built queries like OR queries.
func (s *Store) searchHolonsRaw(ctx context.Context, rawQuery, layerFilter, statusFilter string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	var sqlQuery string
	var args []interface{}

	if layerFilter != "" {
		sqlQuery = `
			SELECT h.id, h.title, h.layer, h.scope, h.cached_r_score, h.updated_at,
			       snippet(holons_fts, 2, '**', '**', '...', 32) as snippet
			FROM holons_fts
			JOIN holons h ON holons_fts.id = h.id
			WHERE holons_fts MATCH ?
			  AND h.layer = ?
			ORDER BY rank
			LIMIT ?
		`
		args = []interface{}{rawQuery, layerFilter, limit}
	} else {
		sqlQuery = `
			SELECT h.id, h.title, h.layer, h.scope, h.cached_r_score, h.updated_at,
			       snippet(holons_fts, 2, '**', '**', '...', 32) as snippet
			FROM holons_fts
			JOIN holons h ON holons_fts.id = h.id
			WHERE holons_fts MATCH ?
			ORDER BY rank
			LIMIT ?
		`
		args = []interface{}{rawQuery, limit}
	}

	rows, err := s.conn.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var updatedAt sql.NullTime
		var rScore sql.NullFloat64
		var scope sql.NullString
		if err := rows.Scan(&r.ID, &r.Title, &r.Layer, &scope, &rScore, &updatedAt, &r.Snippet); err != nil {
			continue
		}
		if scope.Valid {
			r.Scope = scope.String
		}
		r.Type = "holon"
		if rScore.Valid {
			r.RScore = rScore.Float64
		}
		if updatedAt.Valid {
			r.UpdatedAt = updatedAt.Time
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

func (s *Store) searchEvidence(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	sqlQuery := `
		SELECT e.id, e.holon_id, e.type, e.created_at,
		       snippet(evidence_fts, 1, '**', '**', '...', 32) as snippet
		FROM evidence_fts
		JOIN evidence e ON evidence_fts.id = e.id
		WHERE evidence_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`

	rows, err := s.conn.QueryContext(ctx, sqlQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var holonID, evidenceType string
		var createdAt sql.NullTime
		if err := rows.Scan(&r.ID, &holonID, &evidenceType, &createdAt, &r.Snippet); err != nil {
			continue
		}
		r.Type = "evidence"
		r.Title = fmt.Sprintf("%s for %s", evidenceType, holonID)
		if createdAt.Valid {
			r.UpdatedAt = createdAt.Time
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// GetRecentHolons returns the N most recently updated holons.
func (s *Store) GetRecentHolons(ctx context.Context, limit int) ([]Holon, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.conn.QueryContext(ctx, `
		SELECT id, type, kind, layer, title, content, context_id, scope, parent_id,
		       cached_r_score, created_at, updated_at
		FROM holons
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var holons []Holon
	for rows.Next() {
		var h Holon
		if err := rows.Scan(&h.ID, &h.Type, &h.Kind, &h.Layer, &h.Title, &h.Content,
			&h.ContextID, &h.Scope, &h.ParentID, &h.CachedRScore, &h.CreatedAt, &h.UpdatedAt); err != nil {
			continue
		}
		holons = append(holons, h)
	}

	return holons, rows.Err()
}

// GetDecayingEvidence returns evidence expiring within daysAhead days.
func (s *Store) GetDecayingEvidence(ctx context.Context, daysAhead int) ([]Evidence, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT e.id, e.holon_id, e.type, e.content, e.verdict, e.assurance_level,
		       e.carrier_ref, e.valid_until, e.created_at
		FROM evidence e
		LEFT JOIN (
			SELECT evidence_id, MAX(waived_until) as latest_waiver
			FROM waivers
			GROUP BY evidence_id
		) w ON e.id = w.evidence_id
		WHERE e.valid_until IS NOT NULL
		  AND date(e.valid_until) BETWEEN date('now') AND date('now', '+' || ? || ' days')
		  AND (w.latest_waiver IS NULL OR w.latest_waiver < datetime('now'))
		ORDER BY e.valid_until ASC
	`, daysAhead)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evidence []Evidence
	for rows.Next() {
		var e Evidence
		if err := rows.Scan(&e.ID, &e.HolonID, &e.Type, &e.Content, &e.Verdict,
			&e.AssuranceLevel, &e.CarrierRef, &e.ValidUntil, &e.CreatedAt); err != nil {
			continue
		}
		evidence = append(evidence, e)
	}

	return evidence, rows.Err()
}
