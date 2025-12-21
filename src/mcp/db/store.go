package db

import (
	"context"
	"database/sql"
	"fmt"
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
